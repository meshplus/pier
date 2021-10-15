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

	srcIBTPMap  map[string]chan *pb.IBTP
	destIBTPMap map[string]chan *pb.IBTP

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
		srcIBTPMap:      make(map[string]chan *pb.IBTP),
		destIBTPMap:     make(map[string]chan *pb.IBTP),
		mode:            typ,
		ctx:             ctx,
		cancel:          cancel,
	}
	return exchanger, nil
}

func (ex *Exchanger) Start() error {
	// init meta info
	var (
		serviceList []string
		err         error
	)

	ex.srcAdaptName = ex.srcAdapt.Name()
	ex.destAdaptName = ex.destAdapt.Name()

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

	// start get ibtp to channel
	if err := ex.srcAdapt.Start(); err != nil {
		return err
	}

	if err := ex.destAdapt.Start(); err != nil {
		return err
	}
	if repo.UnionMode == ex.mode {
		ex.recover(ex.destServiceMeta, ex.srcServiceMeta)
	} else {
		ex.recover(ex.srcServiceMeta, ex.destServiceMeta)
	}

	// start consumer
	go ex.listenIBTPFromSrcAdaptToServicePairCh()
	go ex.listenIBTPFromDestAdaptToServicePairCh()

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

func (ex *Exchanger) listenIBTPFromDestAdaptToServicePairCh() {
	ex.logger.Infof("listenIBTPFromDestAdaptToServicePairCh %s Start!", ex.destAdaptName)
	ch := ex.destAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("listenIBTPFromDestAdaptToServicePairCh Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			key := ibtp.From + ibtp.To
			_, ok2 := ex.destIBTPMap[key]
			if !ok2 {
				ex.destIBTPMap[key] = make(chan *pb.IBTP, 1024)
				go ex.listenIBTPFromDestAdapt(key)
			}
			ex.destIBTPMap[key] <- ibtp

		}
	}
}
func (ex *Exchanger) listenIBTPFromDestAdapt(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromDestAdapt Stop!")
			return
		case ibtp, ok := <-ex.destIBTPMap[servicePair]:
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

func (ex *Exchanger) listenIBTPFromSrcAdaptToServicePairCh() {
	ex.logger.Infof("listenIBTPFromSrcAdaptToServicePairCh %s Start!", ex.srcAdaptName)
	ch := ex.srcAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("listenIBTPFromSrcAdaptToServicePairCh Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			key := ibtp.From + ibtp.To
			_, ok2 := ex.srcIBTPMap[key]
			if !ok2 {
				ex.srcIBTPMap[key] = make(chan *pb.IBTP, 1024)
				go ex.listenIBTPFromSrcAdapt(key)
			}
			ex.srcIBTPMap[key] <- ibtp

		}
	}
}
func (ex *Exchanger) listenIBTPFromSrcAdapt(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromSrcAdapt Stop!")
			return
		case ibtp, ok := <-ex.srcIBTPMap[servicePair]:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.srcAdaptName)
			var index uint64
			if ex.isIBTPBelongSrc(ibtp) {
				_, ok = ex.srcServiceMeta[ibtp.From]
				if !ok {
					interchain, err := ex.srcAdapt.QueryInterchain(ibtp.From)
					if err != nil {
						ex.logger.Panic(ex.logger)
					}
					ex.srcServiceMeta[ibtp.From] = interchain
				}
				index = ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To]
			} else {
				_, ok = ex.srcServiceMeta[ibtp.To]
				if !ok {
					interchain, err := ex.srcAdapt.QueryInterchain(ibtp.To)
					if err != nil {
						ex.logger.Panic(ex.logger)
					}
					ex.srcServiceMeta[ibtp.To] = interchain
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
