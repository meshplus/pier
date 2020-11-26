package exchanger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"

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

	sendIBTPCounter atomic.Uint64
	sendIBTPTimer   atomic.Duration

	ch       chan struct{}
	ibtps    sync.Map
	receipts sync.Map
	ctx      context.Context
	cancel   context.CancelFunc
}

type Pool struct {
	ibtps *sync.Map
	ch    chan *pb.IBTP
}

func NewPool() *Pool {
	return &Pool{
		ibtps: &sync.Map{},
		ch:    make(chan *pb.IBTP, 40960),
	}
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
		ch:                make(chan struct{}, 100),
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
	}

	logger.Info("Exchanger started")
	return nil
}

func (ex *Exchanger) analysisDirectTPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	current := time.Now()
	counter := ex.sendIBTPCounter.Load()
	for {
		select {
		case <-ticker.C:
			tps := ex.sendIBTPCounter.Load() - counter
			counter = ex.sendIBTPCounter.Load()
			totalTimer := ex.sendIBTPTimer.Load()

			if tps != 0 {
				logger.WithFields(logrus.Fields{
					"tps":      tps,
					"tps_sum":  counter,
					"tps_time": totalTimer.Milliseconds() / int64(counter),
					"tps_avg":  float64(counter) / time.Since(current).Seconds(),
				}).Info("analysis")
			}

		case <-ex.ctx.Done():
			return
		}
	}
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
		"to":    ibtp.To,
		"id":    ibtp.ID(),
	})

	switch ex.mode {
	case repo.UnionMode:
		fallthrough
	case repo.RelayMode:
		if err := retry.Retry(func(attempt uint) error {
			receipt, err := ex.agent.SendIBTP(ibtp)
			if err != nil {
				entry.Errorf("send ibtp to bitxhub: %s", err.Error())
				return fmt.Errorf("send ibtp to bitxhub: %s", err.Error())
			}

			if !receipt.IsSuccess() {
				entry.WithField("error", string(receipt.Ret)).Error("Send ibtp")
				return nil
			}

			entry.WithFields(logrus.Fields{
				"hash": receipt.TxHash.String(),
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

func (ex *Exchanger) feedIBTP(ibtp *pb.IBTP) {
	act, loaded := ex.ibtps.LoadOrStore(ibtp.From, NewPool())
	pool := act.(*Pool)
	pool.feed(ibtp)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					logger.Error(fmt.Errorf("%v", e))
				}
			}()
			inMeta := ex.exec.QueryLatestMeta()
			for ibtp := range pool.ch {
				idx := executor.LoadSyncMap(inMeta, ibtp.From)
				if ibtp.Index <= idx {
					logger.Warn("ignore ibtp with invalid index:{}", ibtp.Index)
					continue
				}
				if idx+1 == ibtp.Index {
					ex.processIBTP(ibtp)
					index := ibtp.Index + 1
					ibtp := pool.get(index)
					for ibtp != nil {
						ex.processIBTP(ibtp)
						index++
						ibtp = pool.get(index)
					}
				} else {
					pool.put(ibtp)
				}
			}
		}(pool)
	}
}

func (ex *Exchanger) processIBTP(ibtp *pb.IBTP) {
	receipt := ex.exec.HandleIBTP(ibtp)
	ex.postHandleIBTP(ibtp.From, receipt)
	ex.sendIBTPCounter.Inc()
}

func (ex *Exchanger) feedReceipt(receipt *pb.IBTP) {
	act, loaded := ex.receipts.LoadOrStore(receipt.To, NewPool())
	pool := act.(*Pool)
	pool.feed(receipt)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					logger.Error(fmt.Errorf("%v", e))
				}
			}()
			callbackMeta := ex.exec.QueryLatestCallbackMeta()
			for ibtp := range pool.ch {
				if ibtp.Index <= executor.LoadSyncMap(callbackMeta, ibtp.To) {
					logger.Warn("ignore ibtp with invalid index")
					continue
				}
				if executor.LoadSyncMap(callbackMeta, ibtp.To)+1 == ibtp.Index {
					ex.exec.HandleIBTP(ibtp)

					index := ibtp.Index + 1
					ibtp := pool.get(index)
					for ibtp != nil {
						receipt := ex.exec.HandleIBTP(ibtp)
						ex.postHandleIBTP(ibtp.From, receipt)
						index++
						ibtp = pool.get(index)
					}
				} else {
					pool.put(ibtp)
				}
			}
		}(pool)
	}
}

func (pool *Pool) feed(ibtp *pb.IBTP) {
	pool.ch <- ibtp
}

func (pool *Pool) put(ibtp *pb.IBTP) {
	pool.ibtps.Store(ibtp.Index, ibtp)
}

func (pool *Pool) get(index uint64) *pb.IBTP {
	ibtp, ok := pool.ibtps.Load(index)
	if !ok {
		return nil
	}

	return ibtp.(*pb.IBTP)
}
