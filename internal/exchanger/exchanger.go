package exchanger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/router"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

var logger = log.NewWithModule("exchanger")

type Exchanger struct {
	mode                 string
	pierID               string
	store                storage.Storage
	mnt                  monitor.Monitor
	exec                 executor.Executor
	syncer               syncer.Syncer
	router               router.Router
	interchainCounter    map[string]uint64
	sourceReceiptCounter map[string]uint64

	apiServer       *api.Server
	peerMgr         peermgr.PeerManager
	checker         checker.Checker
	sendIBTPCounter atomic.Uint64
	sendIBTPTimer   atomic.Duration
	ch              chan struct{} //control the concurrent count
	ibtps           sync.Map
	receipts        sync.Map

	ctx    context.Context
	cancel context.CancelFunc
}

func New(typ, pierID string, meta *pb.Interchain, opts ...Option) (*Exchanger, error) {
	config, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}

	interchainCounter := make(map[string]uint64)
	for id, idx := range meta.InterchainCounter {
		interchainCounter[id] = idx
	}

	sourceReceiptCounter := make(map[string]uint64)
	for id, idx := range meta.SourceReceiptCounter {
		sourceReceiptCounter[id] = idx
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Exchanger{
		checker:              config.checker,
		exec:                 config.exec,
		apiServer:            config.apiServer,
		mnt:                  config.mnt,
		peerMgr:              config.peerMgr,
		syncer:               config.syncer,
		store:                config.store,
		router:               config.router,
		ch:                   make(chan struct{}, 100),
		interchainCounter:    interchainCounter,
		sourceReceiptCounter: sourceReceiptCounter,
		mode:                 typ,
		pierID:               pierID,
		ctx:                  ctx,
		cancel:               cancel,
	}, nil
}

func (ex *Exchanger) Start() error {
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
	if ex.mode != repo.UnionMode {
		go ex.listenAndSendIBTPFromMnt()
	}

	logger.Info("Exchanger started")
	return nil
}

func (ex *Exchanger) startWithDirectMode() error {
	if err := ex.apiServer.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	if err := ex.peerMgr.RegisterConnectHandler(ex.handleNewConnection); err != nil {
		return fmt.Errorf("register on connection handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_INTERCHAIN_META_GET, ex.handleGetInterchainMessage); err != nil {
		return fmt.Errorf("register query interchain msg handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_IBTP_SEND, ex.handleSendIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_IBTP_RECEIPT_SEND, ex.handleSendIBTPReceiptMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_IBTP_GET, ex.handleGetIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp receipt handler: %w", err)
	}

	if err := ex.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	go ex.analysisDirectTPS()
	return nil
}

func (ex *Exchanger) startWithRelayMode() error {
	// recover exchanger before relay any interchain msgs
	ex.recoverRelay()

	if err := ex.syncer.Start(); err != nil {
		return fmt.Errorf("syncer start: %w", err)

	}
	return nil
}

func (ex *Exchanger) startWithUnionMode() error {
	if err := ex.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_ROUTER_IBTP_SEND, ex.handleRouterSendIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp handler: %w", err)
	}

	if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_ROUTER_INTERCHAIN_SEND, ex.handleRouterInterchain); err != nil {
		return fmt.Errorf("register router interchain handler: %w", err)
	}

	if err := ex.syncer.RegisterAppchainHandler(ex.handleProviderAppchains); err != nil {
		return fmt.Errorf("register router handler: %w", err)
	}

	if err := ex.syncer.RegisterRecoverHandler(ex.handleRecover); err != nil {
		return fmt.Errorf("register recover handler: %w", err)
	}

	if err := ex.router.Start(); err != nil {
		return fmt.Errorf("router start: %w", err)
	}

	if err := ex.syncer.Start(); err != nil {
		return fmt.Errorf("syncer start: %w", err)
	}
	return nil
}

func (ex *Exchanger) listenAndSendIBTPFromMnt() {
	ch := ex.mnt.ListenIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			if !ok {
				logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			index := ex.interchainCounter[ibtp.To]
			if index >= ibtp.Index {
				logger.WithFields(logrus.Fields{
					"index":      ibtp.Index,
					"to_counter": index,
					"ibtp_id":    ibtp.ID(),
				}).Info("Ignore ibtp")
				return
			}

			if index+1 < ibtp.Index {
				logger.WithFields(logrus.Fields{
					"index": ibtp.Index,
					"to":    ibtp.To,
				}).Info("Get missing ibtp")

				if err := ex.handleMissingIBTPFromMnt(ibtp.To, index+1, ibtp.Index); err != nil {
					logger.WithFields(logrus.Fields{
						"index": ibtp.Index,
						"to":    ibtp.To,
					}).Error("Handle missing ibtp")
				}
			}

			ex.interchainCounter[ibtp.To] = ibtp.Index

			if err := ex.sendIBTP(ibtp); err != nil {
				logger.Infof("Send ibtp: %s", err.Error())
			}
		}
	}
}

func (ex *Exchanger) listenAndSendIBTPFromSyncer() {
	ch := ex.syncer.ListenIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			if !ok {
				logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			index := ex.interchainCounter[ibtp.From]
			if index >= ibtp.Index {
				logger.WithFields(logrus.Fields{
					"index":      ibtp.Index,
					"from_counter": index,
					"ibtp_id":    ibtp.ID(),
				}).Info("Ignore ibtp")
				return
			}

			if index+1 < ibtp.Index {
				logger.WithFields(logrus.Fields{
					"index": ibtp.Index,
					"from":    ibtp.From,
				}).Info("Get missing ibtp")

				if err := ex.handleMissingIBTPFromSyncer(ibtp.From, index+1, ibtp.Index); err != nil {
					logger.WithFields(logrus.Fields{
						"index": ibtp.Index,
						"from":    ibtp.From,
					}).Error("Handle missing ibtp")
				}
			}

			ex.interchainCounter[ibtp.From] = ibtp.Index

			ex.handleIBTP(ibtp)
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

	logger.Info("Exchanger stopped")

	return nil
}

func (ex *Exchanger) sendIBTP(ibtp *pb.IBTP) error {
	entry := logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"to":    ibtp.To,
		"id":    ibtp.ID(),
	})

	switch ex.mode {
	case repo.UnionMode:
		fallthrough
	case repo.RelayMode:
		err := ex.syncer.SendIBTP(ibtp)
		if err != nil {
			entry.Errorf("send ibtp to bitxhub: %s", err.Error())
			return fmt.Errorf("send ibtp to bitxhub: %s", err.Error())
		}
	case repo.DirectMode:
		// send ibtp to another pier
		if err := retry.Retry(func(attempt uint) error {
			data, err := ibtp.Marshal()
			if err != nil {
				panic(fmt.Sprintf("marshal ibtp: %s", err.Error()))
			}
			msg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, data)

			var dst string
			if ibtp.Type == pb.IBTP_INTERCHAIN {
				dst = ibtp.To
			} else {
				dst = ibtp.From
			}

			if err := ex.peerMgr.AsyncSend(dst, msg); err != nil {
				logger.Errorf("Send ibtp to pier %s: %s", ibtp.ID(), err.Error())
				return err
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	}
	return nil
}

func (ex *Exchanger) queryIBTP(from string, idx uint64) (*pb.IBTP, error) {
	ibtp := &pb.IBTP{}
	id := fmt.Sprintf("%s-%s-%d", from, ex.pierID, idx)

	v := ex.store.Get(model.IBTPKey(id))
	if v != nil {
		if err := ibtp.Unmarshal(v); err != nil {
			return nil, err
		}
		return ibtp, nil
	}

	// query ibtp from counterpart chain
	var err error
	switch ex.mode {
	case repo.RelayMode:
		ibtp, err = ex.syncer.GetIBTPByID(id)
		if err != nil {
			if errors.Is(err, agent.ErrIBTPNotFound) {
				logger.Panicf("query ibtp by id %s from bitxhub: %s", id, err.Error())
			}
			return nil, fmt.Errorf("query ibtp from bitxhub: %s", err.Error())
		}
	case repo.DirectMode:
		// query ibtp from another pier
		msg := peermgr.Message(peerMsg.Message_IBTP_GET, true, []byte(id))
		result, err := ex.peerMgr.Send(from, msg)
		if err != nil {
			return nil, err
		}

		if err := ibtp.Unmarshal(result.Payload.Data); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported pier mode")
	}

	return ibtp, nil
}
