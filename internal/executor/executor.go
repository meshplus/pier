package executor

import (
	"context"
	"fmt"

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
	err := e.client.Unescrow(unLockEvt)
	if err != nil {
		return err
	}
	return nil
}

func (e *ChannelExecutor) QueryFilterLockStart(appchainIndex int64) int64 {
	res := e.client.QueryFilterLockStart(appchainIndex)
	return res
}

func (e *ChannelExecutor) QueryLockEventByIndex(index int64) *pb.LockEvent {
	res := e.client.QueryLockEventByIndex(index)
	return res
}

func (e *ChannelExecutor) QueryAppchainIndex() int64 {
	res := e.client.QueryAppchainIndex()
	return res
}

func (e *ChannelExecutor) QueryRelayIndex() int64 {
	res := e.client.QueryRelayIndex()
	return res
}
