package exchanger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/meshplus/pier/internal/router"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
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
	ctx               context.Context
	cancel            context.CancelFunc
}

func New(typ, pierID string, meta *rpcx.Interchain, opts ...Option) (*Exchanger, error) {
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

		if err := ex.syncer.Start(); err != nil {
			return fmt.Errorf("syncer start: %w", err)
		}

	case repo.UnionMode:
		if err := ex.peerMgr.Start(); err != nil {
			return fmt.Errorf("peerMgr start: %w", err)
		}

		if err := ex.peerMgr.RegisterMsgHandler(peerMsg.Message_ROUTER_IBTP_SEND, ex.handleRouterSendIBTPMessage); err != nil {
			return fmt.Errorf("register ibtp handler: %w", err)
		}

		if err := ex.syncer.RegisterIBTPHandler(ex.handleUnionIBTP); err != nil {
			return fmt.Errorf("register ibtp handler: %w", err)
		}

		if err := ex.syncer.RegisterRouterHandler(ex.handleProviderAppchains); err != nil {
			return fmt.Errorf("register ibtp handler: %w", err)
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
		"to":    types.String2Address(ibtp.To).ShortString(),
		"id":    ibtp.ID(),
	})

	switch ex.mode {
	case repo.UnionMode:
		fallthrough
	case repo.RelayMode:
		if err := retry.Retry(func(attempt uint) error {
			receipt, err := ex.agent.SendIBTP(ibtp)
			if err != nil {
				return fmt.Errorf("send ibtp to bitxhub: %s", err.Error())
			}

			if !receipt.IsSuccess() {
				entry.WithField("error", string(receipt.Ret)).Error("Send ibtp")
				return nil
			}

			entry.WithFields(logrus.Fields{
				"hash": receipt.TxHash.Hex(),
			}).Info("Send ibtp")

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
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
				dst = strings.ToLower(ibtp.To)
			} else {
				dst = strings.ToLower(ibtp.From)
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

			//if err := ex.checker.Check(receipt); err != nil {
			//	logger.Errorf("check ibtp: %s", err.Error())
			//	return err
			//}

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

	v, err := ex.store.Get(model.IBTPKey(id))
	if err == nil {
		if err := ibtp.Unmarshal(v); err != nil {
			return nil, err
		}
		return ibtp, nil
	}

	if err != leveldb.ErrNotFound {
		panic(err)
	}

	// query ibtp from counterpart chain
	loop := func() error {
		switch ex.mode {
		case repo.RelayMode:
			ibtp, err = ex.agent.GetIBTPByID(id)
			if err != nil {
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
