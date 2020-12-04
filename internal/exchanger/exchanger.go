package exchanger

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
)

var logger = log.NewWithModule("exchanger")

type Exchanger struct {
	agent             agent.Agent
	checker           checker.Checker
	store             storage.Storage
	peerMgr           peermgr.PeerManager
	mnt               monitor.Monitor
	exec              executor.Executor
	syncer            syncer.Syncer
	router            router.Router
	apiServer         *api.Server
	mode              string
	pierID            string
	interchainCounter map[string]uint64
	sourceReceiptMeta map[string]uint64
	rollbackCh        chan *pb.IBTP
	ctx               context.Context
	cancel            context.CancelFunc
}

func New(typ, pierID string, meta *pb.Interchain, opts ...Option) (*Exchanger, error) {
	config, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Exchanger{
		agent:             config.agent,
		checker:           config.checker,
		exec:              config.exec,
		apiServer:         config.apiServer,
		mnt:               config.mnt,
		peerMgr:           config.peerMgr,
		syncer:            config.syncer,
		store:             config.store,
		router:            config.router,
		interchainCounter: meta.InterchainCounter,
		sourceReceiptMeta: meta.SourceReceiptCounter,
		rollbackCh:        make(chan *pb.IBTP, 1024),
		mode:              typ,
		pierID:            pierID,
		ctx:               ctx,
		cancel:            cancel,
	}, nil
}

func (ex *Exchanger) Start() error {
	switch ex.mode {
	case repo.DirectMode:
		// start some extra module for direct link mode
		if err := ex.apiServer.Start(); err != nil {
			return fmt.Errorf("peerMgr start: %w", err)
		}

		if err := ex.peerMgr.Start(); err != nil {
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
		if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_IBTP_GET, ex.handleGetIBTPMessage); err != nil {
			return fmt.Errorf("register ibtp receipt handler: %w", err)
		}
	case repo.RelayMode:
		// recover exchanger before relay any interchain msgs
		ex.recoverRelay()

		if err := ex.syncer.RegisterIBTPHandler(ex.handleIBTP); err != nil {
			return fmt.Errorf("register ibtp handler: %w", err)
		}

		if err := ex.syncer.RegisterRollbackHandler(ex.handleRollback); err != nil {
			return fmt.Errorf("register rollback handler: %w", err)
		}

		if err := ex.syncer.Start(); err != nil {
			return fmt.Errorf("syncer start: %w", err)
		}

	case repo.UnionMode:
		if err := ex.peerMgr.Start(); err != nil {
			return fmt.Errorf("peerMgr start: %w", err)
		}

		if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_ROUTER_IBTP_SEND, ex.handleRouterSendIBTPMessage); err != nil {
			return fmt.Errorf("register router ibtp handler: %w", err)
		}

		if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_ROUTER_INTERCHAIN_SEND, ex.handleRouterInterchain); err != nil {
			return fmt.Errorf("register router interchain handler: %w", err)
		}

		if err := ex.syncer.RegisterIBTPHandler(ex.handleUnionIBTP); err != nil {
			return fmt.Errorf("register ibtp handler: %w", err)
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
	}

	if ex.mode != repo.UnionMode {
		go ex.listenAndSendIBTP()
		go ex.sendRollbackedIBTP()
	}

	logger.Info("Exchanger started")
	return nil
}

func (ex *Exchanger) listenAndSendIBTP() {
	ch := ex.mnt.ListenOnIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ch:
			if !ok {
				logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			if err := ex.sendIBTP(ibtp); err != nil {
				logger.Infof("Send ibtp: %s", err.Error())
			}
		}
	}
}

func (ex *Exchanger) sendRollbackedIBTP() {
	for {
		select {
		case <-ex.ctx.Done():
			return
		case ibtp, ok := <-ex.rollbackCh:
			if !ok {
				logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ibtp.Type = pb.IBTP_ROLLBACK
			if err := ex.sendIBTP(ibtp); err != nil {
				logger.Infof("Send rollbacked ibtp: %s", err.Error())
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

	logger.Info("Exchanger stopped")

	return nil
}

func (ex *Exchanger) sendIBTP(ibtp *pb.IBTP) error {
	entry := logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	})

	switch ex.mode {
	case repo.UnionMode:
		fallthrough
	case repo.RelayMode:
		strategies := []strategy.Strategy{strategy.Wait(2 * time.Second)}
		if ibtp.Type != pb.IBTP_ROLLBACK {
			strategies = append(strategies, strategy.Limit(5))
		}
		if err := retry.Retry(func(attempt uint) error {
			receipt, err := ex.agent.SendIBTP(ibtp)
			if err != nil {
				entry.Errorf("send ibtp to bitxhub: %s", err.Error())
				return fmt.Errorf("send ibtp to bitxhub: %s", err.Error())
			}

			if !receipt.IsSuccess() {
				// If the sending index is larger than the index in BitXHub, means there are some timeout
				// ibtps to send to bitxhub, so retry until those ibtps sending finished
				if strings.Contains(string(receipt.Ret), "wrong index") {
					entry.WithField("error", string(receipt.Ret)).Error("Send ibtp get wrong index")
					return fmt.Errorf("send ibtp get wrong index")
				}
				entry.WithField("error", string(receipt.Ret)).Error("Send ibtp")
				return nil
			}

			entry.WithFields(logrus.Fields{
				"hash": receipt.TxHash.String(),
			}).Info("Send ibtp")

			return nil
		}, strategies...); err != nil {
			ex.exec.Rollback(ibtp, true)
			ex.rollbackCh <- ibtp
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

			retMsg, err := ex.peerMgr.Send(dst, msg)
			if err != nil {
				logger.Infof("Send ibtp to pier %s: %s", dst, err.Error())
				return err
			}

			if !retMsg.Payload.Ok {
				entry.Errorf("send ibtp: %s", string(retMsg.Payload.Data))
				return fmt.Errorf("send ibtp: %w", fmt.Errorf(string(retMsg.Payload.Data)))
			}

			entry.WithFields(logrus.Fields{
				"status": retMsg.Payload.Ok,
			}).Info("Send ibtp")

			// ignore msg for receipt type
			if ibtp.Type == pb.IBTP_RECEIPT_SUCCESS || ibtp.Type == pb.IBTP_RECEIPT_FAILURE {
				return nil
			}

			// handle receipt message from pier
			receipt := &pb.IBTP{}
			if err := receipt.Unmarshal(retMsg.Payload.Data); err != nil {
				logger.Errorf("unmarshal receipt: %s", err.Error())
				return err
			}
			ex.exec.HandleIBTP(receipt)
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
	loop := func() error {
		var err error
		switch ex.mode {
		case repo.RelayMode:
			ibtp, err = ex.agent.GetIBTPByID(id)
			if err != nil {
				if errors.Is(err, agent.ErrIBTPNotFound) {
					logger.Panicf("query ibtp by id %s from bitxhub: %s", id, err.Error())
				}
				return fmt.Errorf("query ibtp from bitxhub: %s", err.Error())
			}
		case repo.DirectMode:
			// query ibtp from another pier
			msg := peermgr.Message(peerMsg.Message_IBTP_GET, true, []byte(id))
			result, err := ex.peerMgr.Send(from, msg)
			if err != nil {
				return err
			}

			if err := ibtp.Unmarshal(result.Payload.Data); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported pier mode")
		}
		return nil
	}

	if err := retry.Retry(func(attempt uint) error {
		return loop()
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}

	return ibtp, nil
}
