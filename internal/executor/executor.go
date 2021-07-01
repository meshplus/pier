package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ Executor = (*ChannelExecutor)(nil)

// ChannelExecutor represents the necessary data for executing interchain txs in appchain
type ChannelExecutor struct {
	client      plugins.Client // the client to interact with appchain
	storage     storage.Storage
	appchainDID string // appchain did
	cryptor     txcrypto.Cryptor
	logger      logrus.FieldLogger
	ctx         context.Context
	cancel      context.CancelFunc
}

// New creates new instance of Executor. agent is for interacting with counterpart chain
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func New(client plugins.Client, appchainDID string, storage storage.Storage, cryptor txcrypto.Cryptor, logger logrus.FieldLogger) (*ChannelExecutor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:      client,
		ctx:         ctx,
		cancel:      cancel,
		storage:     storage,
		appchainDID: appchainDID,
		cryptor:     cryptor,
		logger:      logger,
	}, nil
}

// Start implements Executor
func (e *ChannelExecutor) Start() error {
	e.logger.Info("Executor started")
	return nil
}

// Stop implements Executor
func (e *ChannelExecutor) Stop() error {
	e.cancel()

	e.logger.Info("Executor stopped")
	return nil
}

func (e *ChannelExecutor) QueryInterchainMeta() map[string]uint64 {
	execMeta, err := e.client.GetInMeta()
	if err != nil {
		return map[string]uint64{}
	}
	return execMeta
}

func (e *ChannelExecutor) QueryCallbackMeta() map[string]uint64 {
	callbackMeta, err := e.client.GetCallbackMeta()
	if err != nil {
		return map[string]uint64{}
	}
	return callbackMeta
}

// getReceipt only generates one receipt given source chain id and interchain tx index
func (e *ChannelExecutor) QueryIBTPReceipt(originalIBTP *pb.IBTP) (*pb.IBTP, error) {
	if originalIBTP == nil {
		return nil, fmt.Errorf("empty original ibtp")
	}
	return e.client.GetReceipt(originalIBTP)
}

func (e *ChannelExecutor) SendBurnEvent(unLockEvt *pb.UnLock) error {
	e.retryFunc(func(attempt uint) error {
		err := e.client.Unescrow(unLockEvt)
		if err != nil {
			return err
		}
		return nil
	})
	return nil
}

func (e *ChannelExecutor) retryFunc(handle func(uint) error) error {
	return retry.Retry(func(attempt uint) error {
		if err := handle(attempt); err != nil {
			e.logger.Errorf("retry failed for reason: %s", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(500*time.Millisecond))
}

func (e *ChannelExecutor) QueryFilterLockStart(appchainIndex uint64) uint64 {
	var (
		res uint64
		err error
	)
	e.retryFunc(func(attempt uint) error {
		res, err = e.client.QueryFilterLockStart(appchainIndex)
		if err != nil {
			return err
		}
		return nil
	})
	return res
}

func (e *ChannelExecutor) QueryLockEventByIndex(index uint64) *pb.LockEvent {
	var (
		res *pb.LockEvent
		err error
	)
	e.retryFunc(func(attempt uint) error {
		res, err = e.client.QueryLockEventByIndex(index)
		if err != nil {
			return err
		}
		return nil
	})
	return res
}

func (e *ChannelExecutor) QueryAppchainIndex() uint64 {
	var (
		res uint64
		err error
	)
	e.retryFunc(func(attempt uint) error {
		res, err = e.client.QueryAppchainIndex()
		if err != nil {
			return err
		}
		return nil
	})
	return res
}

func (e *ChannelExecutor) QueryRelayIndex() uint64 {
	var (
		res uint64
		err error
	)
	e.retryFunc(func(attempt uint) error {
		res, err = e.client.QueryRelayIndex()
		if err != nil {
			return err
		}
		return nil
	})
	return res
}
