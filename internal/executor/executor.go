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
	client     plugins.Client // the client to interact with appchain
	storage    storage.Storage
	appchainID string // appchain id
	cryptor    txcrypto.Cryptor
	logger     logrus.FieldLogger
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates new instance of Executor. agent is for interacting with counterpart chain
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func New(client plugins.Client, appchainDID string, storage storage.Storage, cryptor txcrypto.Cryptor, logger logrus.FieldLogger) (*ChannelExecutor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:     client,
		ctx:        ctx,
		cancel:     cancel,
		storage:    storage,
		appchainID: appchainDID,
		cryptor:    cryptor,
		logger:     logger,
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
		return make(map[string]uint64)
	}
	if execMeta == nil {
		execMeta = make(map[string]uint64)
	}

	return execMeta
}

func (e *ChannelExecutor) QueryCallbackMeta() map[string]uint64 {
	callbackMeta, err := e.client.GetCallbackMeta()
	if err != nil {
		return make(map[string]uint64)
	}
	if callbackMeta == nil {
		callbackMeta = make(map[string]uint64)
	}

	return callbackMeta
}

func (e *ChannelExecutor) QueryDstRollbackMeta() map[string]uint64 {
	dstRollbackMeta, err := e.client.GetDstRollbackMeta()
	if err != nil {
		return make(map[string]uint64)
	}
	if dstRollbackMeta == nil {
		dstRollbackMeta = make(map[string]uint64)
	}

	return dstRollbackMeta
}

func (e *ChannelExecutor) QueryIBTPReceipt(originalIBTP *pb.IBTP) (*pb.IBTP, error) {
	if originalIBTP == nil {
		return nil, fmt.Errorf("empty original ibtp")
	}
	return e.client.GetReceiptMessage(pb.GenServicePair(originalIBTP.From, originalIBTP.To), originalIBTP.Index)
}
