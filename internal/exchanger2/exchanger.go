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
	list0 := ex.srcAdapt.GetServiceIDList()
	for _, serviceId := range list0 {
		ex.srcServiceMeta[serviceId] = ex.srcAdapt.QueryInterchain(serviceId)
		ex.destServiceMeta[serviceId] = ex.destAdapt.QueryInterchain(serviceId)
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

// 从中继链监听的ibtp：可以收到srcAdapt作为来源链的回执 和 作为目的链的请求 以及 （作为来源链的请求rollback）
func (ex *Exchanger) listenIBTPFromDestAdapt() {
	ch := ex.destAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			ex.logger.Info("Receive ibtp from :", ex.destAdapt.Name())
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			var index uint64
			if ex.isIBTPBelongSrc(ibtp) {
				_, ok = ex.destServiceMeta[ibtp.From]
				if !ok {
					ex.destServiceMeta[ibtp.From] = &pb.Interchain{
						ID:                      ibtp.From,
						InterchainCounter:       make(map[string]uint64),
						ReceiptCounter:          make(map[string]uint64),
						SourceInterchainCounter: make(map[string]uint64),
						SourceReceiptCounter:    make(map[string]uint64),
					}
				}
				index = ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To]
			} else {
				_, ok = ex.destServiceMeta[ibtp.To]
				if !ok {
					ex.destServiceMeta[ibtp.To] = &pb.Interchain{
						ID:                      ibtp.To,
						InterchainCounter:       make(map[string]uint64),
						ReceiptCounter:          make(map[string]uint64),
						SourceInterchainCounter: make(map[string]uint64),
						SourceReceiptCounter:    make(map[string]uint64),
					}
				}
				index = ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From]
			}

			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				return
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.destAdapt, ex.srcAdapt, ibtp.From, ibtp.To, !ex.isIBTPBelongSrc(ibtp))
			}

			if err := retry.Retry(func(attempt uint) error {
				if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
					ex.logger.Errorf("Send ibtp: %s", err.Error())
					// todo: ??? if err occurs, try to get new ibtp and resend
					return fmt.Errorf("retry sending ibtp")
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

// 可以收到srcAdapt作为来源链的请求 和 作为目的链的回执
func (ex *Exchanger) listenIBTPFromSrcAdapt() {
	ch := ex.srcAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			ex.logger.Info("Receive interchain ibtp from :", ex.srcAdapt.Name())
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			var index uint64
			if ex.isIBTPBelongSrc(ibtp) {
				_, ok = ex.srcServiceMeta[ibtp.From]
				if !ok {
					ex.srcServiceMeta[ibtp.From] = &pb.Interchain{
						ID:                      ibtp.From,
						InterchainCounter:       make(map[string]uint64),
						ReceiptCounter:          make(map[string]uint64),
						SourceInterchainCounter: make(map[string]uint64),
						SourceReceiptCounter:    make(map[string]uint64),
					}
				}
				index = ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To]
			} else {
				_, ok = ex.srcServiceMeta[ibtp.To]
				if !ok {
					ex.srcServiceMeta[ibtp.To] = &pb.Interchain{
						ID:                      ibtp.To,
						InterchainCounter:       make(map[string]uint64),
						ReceiptCounter:          make(map[string]uint64),
						SourceInterchainCounter: make(map[string]uint64),
						SourceReceiptCounter:    make(map[string]uint64),
					}
				}
				index = ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From]
			}

			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				return
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.srcAdapt, ex.destAdapt, ibtp.From, ibtp.To, ex.isIBTPBelongSrc(ibtp))
			}

			if err := retry.Retry(func(attempt uint) error {
				if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
					ex.logger.Errorf("Send ibtp: %s", err.Error())
					// todo: ??? if err occurs, try to get new ibtp and resend
					return fmt.Errorf("retry sending ibtp")
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
