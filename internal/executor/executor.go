package executor

import (
	"context"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
)

var (
	_      Executor = (*ChannelExecutor)(nil)
	logger          = log.NewWithModule("executor")
)

// ChannelExecutor represents the necessary data for executing interchain txs in appchain
type ChannelExecutor struct {
	client  plugins.Client // the client to interact with appchain
	storage storage.Storage
	id      string // appchain id
	cryptor txcrypto.Cryptor
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates new instance of Executor. agent is for interacting with counterpart chain
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func New(client plugins.Client, pierID string, storage storage.Storage, cryptor txcrypto.Cryptor) (*ChannelExecutor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
		storage: storage,
		id:      pierID,
		cryptor: cryptor,
	}, nil
}

// Start implements Executor
func (e *ChannelExecutor) Start() error {
	logger.Info("Executor started")
	return nil
}

// Stop implements Executor
func (e *ChannelExecutor) Stop() error {
	e.cancel()

	logger.Info("Executor stopped")
	return nil
}

func (e *ChannelExecutor) QueryMeta() map[string]uint64 {
	execMeta, err := e.client.GetInMeta()
	if err != nil {
		return map[string]uint64{}
	}
	return execMeta
}

// getReceipt only generates one receipt given source chain id and interchain tx index
func (e *ChannelExecutor) QueryIBTPReceipt(from string, idx uint64, originalIBTP *pb.IBTP) *pb.IBTP {
	ret, err := e.client.GetInMessage(from, idx)
	if err != nil {
		return nil
	}

	// todo: add error handling
	ibtpReceipt, _ := e.generateCallback(originalIBTP, ret)
	return ibtpReceipt
}
