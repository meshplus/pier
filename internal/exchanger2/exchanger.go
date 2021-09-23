package exchanger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

type Exchanger struct {
	mode       string
	srcChainId string
	srcBxhId   string
	// R: appchain -- hub
	// D: appchain -- dPier
	// U: hub 	   -- uPier
	srcAdapt        adapt.Adapt
	destAdapt       adapt.Adapt
	srcAdaptName    string
	destAdaptName   string
	srcServiceMeta  map[string]*pb.Interchain
	destServiceMeta map[string]*pb.Interchain

	logger logrus.FieldLogger
	ctx    context.Context
	cancel context.CancelFunc
}

func New(typ, srcChainId, srcBxhId string, opts ...Option) (*Exchanger, error) {
	config := GenerateConfig(opts...)

	ctx, cancel := context.WithCancel(context.Background())
	exchanger := &Exchanger{
		srcChainId:      srcChainId,
		srcBxhId:        srcBxhId,
		srcAdapt:        config.srcAdapt,
		destAdapt:       config.destAdapt,
		logger:          config.logger,
		srcServiceMeta:  make(map[string]*pb.Interchain),
		destServiceMeta: make(map[string]*pb.Interchain),
		mode:            typ,
		ctx:             ctx,
		cancel:          cancel,
	}
	return exchanger, nil
}

func (ex *Exchanger) Start() error {
	// init meta info
	var (
		srcAdaptName  string
		destAdaptName string
		serviceList   []string
		err           error
	)

	if err := retry.Retry(func(attempt uint) error {
		if srcAdaptName, err = ex.srcAdapt.Name(); err != nil {
			ex.logger.Errorf("get name from srcAdapt", "error", err.Error())
			return err
		}
		ex.srcAdaptName = srcAdaptName
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to get name from srcAdapt: %w", err)
	}
	if err := retry.Retry(func(attempt uint) error {
		if destAdaptName, err = ex.destAdapt.Name(); err != nil {
			ex.logger.Errorf("get name from destAdapt", "error", err.Error())
			return err
		}
		ex.destAdaptName = destAdaptName
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to get name from destAdapt: %w", err)
	}

	if err := retry.Retry(func(attempt uint) error {
		if serviceList, err = ex.srcAdapt.GetServiceIDList(); err != nil {
			ex.logger.Errorf("get serviceIdList from srcAdapt", "error", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to get serviceIdList from srcAdapt: %w", err)
	}

	for _, serviceId := range serviceList {
		ex.srcServiceMeta[serviceId], err = ex.srcAdapt.QueryInterchain(serviceId)
		if err != nil {
			panic(fmt.Sprintf("queryInterchain from srcAdapt: %s", err.Error()))
		}
		ex.destServiceMeta[serviceId], err = ex.destAdapt.QueryInterchain(serviceId)
		if err != nil {
			panic(fmt.Sprintf("queryInterchain from destAdapt: %s", err.Error()))
		}
	}

	ex.recover()

	// start get ibtp to channel
	ex.srcAdapt.Start()
	ex.destAdapt.Start()

	// start consumer
	go ex.listenIBTPFromSrcAdapt()
	go ex.listenIBTPFromDestAdapt()

	ex.logger.Info("Exchanger started")
	return nil
}

func initInterchain(serviceMeta map[string]*pb.Interchain, fullServiceId string) {
	serviceMeta[fullServiceId] = &pb.Interchain{
		ID:                      fullServiceId,
		InterchainCounter:       make(map[string]uint64),
		ReceiptCounter:          make(map[string]uint64),
		SourceInterchainCounter: make(map[string]uint64),
		SourceReceiptCounter:    make(map[string]uint64),
	}
}

func (ex *Exchanger) listenIBTPFromDestAdapt() {
	ch := ex.destAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromDestAdapt Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.destAdaptName)
			var index uint64
			if ex.isIBTPBelongSrc(ibtp) {
				_, ok = ex.destServiceMeta[ibtp.From]
				if !ok {
					initInterchain(ex.destServiceMeta, ibtp.From)
				}
				index = ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To]
			} else {
				_, ok = ex.destServiceMeta[ibtp.To]
				if !ok {
					initInterchain(ex.destServiceMeta, ibtp.To)
				}
				index = ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From]
			}

			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.destAdapt, ex.srcAdapt, ibtp.From, ibtp.To, !ex.isIBTPBelongSrc(ibtp))
			}

			if err := retry.Retry(func(attempt uint) error {
				if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To] = ibtp.Index
			} else {
				ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) listenIBTPFromSrcAdapt() {
	ch := ex.srcAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromSrcAdapt Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.srcAdaptName)
			var index uint64
			if ex.isIBTPBelongSrc(ibtp) {
				_, ok = ex.srcServiceMeta[ibtp.From]
				if !ok {
					initInterchain(ex.srcServiceMeta, ibtp.From)
				}
				index = ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To]
			} else {
				_, ok = ex.srcServiceMeta[ibtp.To]
				if !ok {
					initInterchain(ex.srcServiceMeta, ibtp.To)
				}
				index = ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From]
			}

			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.srcAdapt, ex.destAdapt, ibtp.From, ibtp.To, ex.isIBTPBelongSrc(ibtp))
			}

			if err := retry.Retry(func(attempt uint) error {
				if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.destAdaptName, "error", err.Error())
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To] = ibtp.Index
			} else {
				ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) isIBTPBelongSrc(ibtp *pb.IBTP) bool {
	var isIBTPBelongSrc = false
	bxhID, chainID, _ := ibtp.ParseFrom()

	switch ex.mode {
	case repo.DirectMode:
		fallthrough
	case repo.RelayMode:
		if strings.EqualFold(ex.srcChainId, chainID) {
			isIBTPBelongSrc = true
		}
	case repo.UnionMode:
		if strings.EqualFold(ex.srcBxhId, bxhID) {
			isIBTPBelongSrc = true
		}
	}
	return isIBTPBelongSrc
}

func (ex *Exchanger) queryIBTP(destAdapt adapt.Adapt, ibtpID string, isReq bool) *pb.IBTP {
	var (
		ibtp *pb.IBTP
		err  error
	)
	if err := retry.Retry(func(attempt uint) error {
		ibtp, err = destAdapt.QueryIBTP(ibtpID, isReq)
		if err != nil {
			ex.logger.Errorf("queryIBTP from Adapt:%s", ex.destAdaptName, "error", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		ex.logger.Panic(err)
	}
	return ibtp
}

func (ex *Exchanger) Stop() error {
	ex.cancel()

	if err := ex.srcAdapt.Stop(); err != nil {
		return fmt.Errorf("srcAdapt stop: %w", err)
	}
	if err := ex.destAdapt.Stop(); err != nil {
		return fmt.Errorf("destAdapt stop: %w", err)
	}
	ex.logger.Info("Exchanger stopped")

	return nil
}
