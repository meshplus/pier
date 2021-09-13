package exchanger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/router"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

type Exchanger struct {
	mode         string
	appchainDID  string
	store        storage.Storage
	mnt          monitor.Monitor
	exec         executor.Executor
	syncer       syncer.Syncer
	router       router.Router
	serviceMeta  map[string]*pb.Interchain
	callbackMeta map[string]uint64
	inMeta       map[string]uint64
	bxhID        string

	apiServer       *api.Server
	peerMgr         peermgr.PeerManager
	checker         checker.Checker
	sendIBTPCounter atomic.Uint64
	sendIBTPTimer   atomic.Duration
	ch              chan struct{} //control the concurrent count
	ibtps           sync.Map
	receipts        sync.Map
	rollbackCh      chan *pb.IBTP

	logger logrus.FieldLogger
	ctx    context.Context
	cancel context.CancelFunc
}

func New(typ, appchainDID string, serviceMeta map[string]*pb.Interchain, opts ...Option) (*Exchanger, error) {
	config := GenerateConfig(opts...)

	ctx, cancel := context.WithCancel(context.Background())
	exchanger := &Exchanger{
		checker:     config.checker,
		apiServer:   config.apiServer,
		mnt:         config.mnt,
		peerMgr:     config.peerMgr,
		syncer:      config.syncer,
		store:       config.store,
		router:      config.router,
		logger:      config.logger,
		ch:          make(chan struct{}, 100),
		serviceMeta: serviceMeta,
		rollbackCh:  make(chan *pb.IBTP, 1024),
		mode:        typ,
		appchainDID: appchainDID,
		ctx:         ctx,
		cancel:      cancel,
	}

	if typ != repo.UnionMode {
		exchanger.exec = config.exec
		exchanger.callbackMeta = config.exec.QueryCallbackMeta()
		exchanger.inMeta = config.exec.QueryInterchainMeta()
	}

	return exchanger, nil
}

func (ex *Exchanger) Start() error {
	if ex.mode != repo.UnionMode {
		go ex.listenAndSendIBTPFromMnt()
	}
	if ex.mode != repo.DirectMode {
		go ex.listenAndSendIBTPFromSyncer()
	}

	var err error
	switch ex.mode {
	case repo.DirectMode:
		err = ex.startWithDirectMode()
	case repo.RelayMode:
		err = ex.startWithRelayMode()
	case repo.UnionMode:
		err = ex.startWithUnionMode()
	}

	if err != nil {
		return err
	}

	ex.logger.Info("Exchanger started")
	return nil
}

func (ex *Exchanger) startWithDirectMode() error {
	if err := ex.apiServer.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	if err := ex.peerMgr.RegisterConnectHandler(ex.handleNewConnection); err != nil {
		return fmt.Errorf("register on connection handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_INTERCHAIN_META_GET, ex.handleGetInterchainMessage); err != nil {
		return fmt.Errorf("register query interchain msg handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_IBTP_SEND, ex.handleSendIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_IBTP_RECEIPT_SEND, ex.handleSendIBTPReceiptMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_IBTP_GET, ex.handleGetIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp receipt handler: %w", err)
	}

	if err := ex.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	go ex.analysisDirectTPS()
	return nil
}

func (ex *Exchanger) startWithRelayMode() error {
	if err := ex.syncer.RegisterRollbackHandler(ex.handleRollback); err != nil {
		return fmt.Errorf("register router handler: %w", err)
	}
	// syncer should be started first in case to recover ibtp from monitor
	if err := ex.syncer.Start(); err != nil {
		return fmt.Errorf("syncer start: %w", err)
	}

	// recover exchanger before relay any interchain msgs
	ex.recoverRelay()

	go ex.sendRollbackedIBTP()

	return nil
}

func (ex *Exchanger) startWithUnionMode() error {
	if err := ex.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_SEND, ex.handleRouterSendIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp send handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_GET, ex.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp get handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_RECEIPT_GET, ex.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp get handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_INTERCHAIN_GET, ex.handleRouterInterchain); err != nil {
		return fmt.Errorf("register router interchain handler: %w", err)
	}

	if err := ex.recoverUnion(); err != nil {
		return fmt.Errorf("recover union: %w", err)
	}

	if err := ex.router.Start(); err != nil {
		return fmt.Errorf("router start: %w", err)
	}

	if err := ex.syncer.Start(); err != nil {
		return fmt.Errorf("syncer start: %w", err)
	}
	return nil
}

func (ex *Exchanger) sendRollbackedIBTP() {
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ex.rollbackCh:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ibtp.Type = pb.IBTP_ROLLBACK
			if err := ex.sendIBTP(ibtp); err != nil {
				ex.logger.Infof("Send rollbacked ibtp: %s", err.Error())
			}
		}
	}
}

func (ex *Exchanger) listenAndSendIBTPFromMnt() {
	ch := ex.mnt.ListenIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			ex.logger.Info("Receive interchain ibtp from monitor")
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			_, ok = ex.serviceMeta[ibtp.From]
			if !ok {
				ex.serviceMeta[ibtp.From] = &pb.Interchain{
					ID:                      ibtp.From,
					InterchainCounter:       make(map[string]uint64),
					ReceiptCounter:          make(map[string]uint64),
					SourceInterchainCounter: make(map[string]uint64),
					SourceReceiptCounter:    make(map[string]uint64),
				}
			}
			index := ex.serviceMeta[ibtp.From].InterchainCounter[ibtp.To]
			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				return
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")

				servicePair := fmt.Sprintf("%s-%s", ibtp.From, ibtp.To)
				if err := ex.handleMissingIBTPFromMnt(servicePair, index+1, ibtp.Index); err != nil {
					ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To, "err": err.Error()}).Error("Handle missing ibtp")
				}
			}

			if err := retry.Retry(func(attempt uint) error {
				if err := ex.sendIBTP(ibtp); err != nil {
					ex.logger.Errorf("Send ibtp: %s", err.Error())
					// if err occurs, try to get new ibtp and resend
					ibtpID := ibtp.ID()
					if err := retry.Retry(func(attempt uint) error {
						ibtp, err = ex.mnt.QueryIBTP(ibtpID)
						if err != nil {
							ex.logger.Errorf("Query ibtp %s from appchain: %s", ibtpID, err.Error())
							return err
						}
						return nil
					}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
						ex.logger.Panic(err)
					}
					return fmt.Errorf("retry sending ibtp")
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			ex.serviceMeta[ibtp.From].InterchainCounter[ibtp.To] = ibtp.Index
		}
	}
}

func (ex *Exchanger) listenAndSendIBTPFromSyncer() {
	ch := ex.syncer.ListenIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case wIbtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			entry := ex.logger.WithFields(logrus.Fields{"type": wIbtp.Ibtp.Type, "id": wIbtp.Ibtp.ID()})
			entry.Debugf("Exchanger receives ibtp from syncer")
			if ex.mode == repo.UnionMode {
				ex.handleUnionIBTPFromBitXHub(wIbtp)
			} else {
				switch wIbtp.Ibtp.Type {
				case pb.IBTP_INTERCHAIN, pb.IBTP_ROLLBACK:
					ex.applyInterchain(wIbtp, entry)
				case pb.IBTP_RECEIPT_SUCCESS, pb.IBTP_RECEIPT_FAILURE, pb.IBTP_RECEIPT_ROLLBACK:
					//ex.applyReceipt(wIbtp, entry)
					ex.feedIBTPReceipt(wIbtp)
				default:
					entry.Errorf("wrong type of ibtp")
				}
			}
		}
	}
}

func (ex *Exchanger) Stop() error {
	ex.cancel()

	switch ex.mode {
	case repo.DirectMode:
		if err := ex.apiServer.Stop(); err != nil {
			return fmt.Errorf("gin service stop: %w", err)
		}
		if err := ex.peerMgr.Stop(); err != nil {
			return fmt.Errorf("peerMgr stop: %w", err)
		}
	case repo.RelayMode:
		if err := ex.syncer.Stop(); err != nil {
			return fmt.Errorf("syncer stop: %w", err)
		}
	case repo.UnionMode:
		if err := ex.syncer.Stop(); err != nil {
			return fmt.Errorf("syncer stop: %w", err)
		}
		if err := ex.peerMgr.Stop(); err != nil {
			return fmt.Errorf("peerMgr stop: %w", err)
		}
		if err := ex.router.Stop(); err != nil {
			return fmt.Errorf("router stop:%w", err)
		}
	}

	ex.logger.Info("Exchanger stopped")

	return nil
}

func (ex *Exchanger) sendIBTP(ibtp *pb.IBTP) error {
	entry := ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "to": ibtp.To, "id": ibtp.ID()})

	switch ex.mode {
	case repo.UnionMode:
		ex.syncer.SendIBTPWithRetry(ibtp)
	case repo.RelayMode:
		err := ex.syncer.SendIBTP(ibtp)
		if err != nil {
			entry.Errorf("Send ibtp to bitxhub: %s", err.Error())
			if errors.Is(err, syncer.ErrMetaOutOfDate) {
				ex.updateInterchainMeta(ibtp.From)
				return nil
			}
			ex.handleRollback(ibtp, "")
			ex.rollbackCh <- ibtp
		}
	case repo.DirectMode:
		// send ibtp to another pier
		if err := retry.Retry(func(attempt uint) error {
			data, err := ibtp.Marshal()
			if err != nil {
				panic(fmt.Sprintf("marshal ibtp: %s", err.Error()))
			}
			msg := peermgr.Message(pb.Message_IBTP_SEND, true, data)

			var dst string
			if ibtp.Type == pb.IBTP_INTERCHAIN {
				dst = ibtp.To
			} else {
				dst = ibtp.From
			}

			if err := ex.peerMgr.AsyncSend(dst, msg); err != nil {
				ex.logger.Errorf("Send ibtp to pier %s: %s", ibtp.ID(), err.Error())
				return err
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			ex.logger.Panic(err)
		}
	}
	entry.Info("Send ibtp success from monitor")
	return nil
}

func (ex *Exchanger) queryIBTP(id, target string, isReq bool) (*pb.IBTP, bool, error) {
	verifiedTx := &pb.VerifiedTx{}
	v := ex.store.Get(model.IBTPKey(id))
	if v != nil {
		if err := verifiedTx.Unmarshal(v); err != nil {
			return nil, false, err
		}
		return verifiedTx.Tx.GetIBTP(), verifiedTx.Valid, nil
	}

	// query ibtp from counterpart chain
	var (
		ibtp    *pb.IBTP
		isValid bool
		err     error
	)
	switch ex.mode {
	case repo.RelayMode:
		ibtp, isValid, err = ex.syncer.QueryIBTP(id, isReq)
		if err != nil {
			if errors.Is(err, syncer.ErrIBTPNotFound) {
				ex.logger.Panicf("query ibtp by id %s from bitxhub: %s", id, err.Error())
			}
			return nil, false, fmt.Errorf("query ibtp from bitxhub: %s", err.Error())
		}
	case repo.DirectMode:
		// query ibtp from another pier
		msg := peermgr.Message(pb.Message_IBTP_GET, true, []byte(id))
		result, err := ex.peerMgr.Send(target, msg)
		if err != nil {
			return nil, false, err
		}

		ibtp = &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(result).Data); err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("unsupported pier mode")
	}

	return ibtp, isValid, nil
}

func copyCounterMap(original map[string]uint64) map[string]uint64 {
	ret := make(map[string]uint64, len(original))
	for id, idx := range original {
		ret[id] = idx
	}
	return ret
}
