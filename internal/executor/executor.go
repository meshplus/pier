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
	if execMeta == nil || err != nil {
		return map[string]uint64{}
	}
	return execMeta
}

func (e *ChannelExecutor) QueryCallbackMeta() map[string]uint64 {
	callbackMeta, err := e.client.GetCallbackMeta()
	if callbackMeta == nil || err != nil {
		return map[string]uint64{}
	}
	return callbackMeta
}

// getReceipt only generates one receipt given source chain id and interchain tx index
func (e *ChannelExecutor) QueryIBTPReceipt(originalIBTP *pb.IBTP) (*pb.IBTP, error) {
	if originalIBTP == nil {
		return nil, fmt.Errorf("empty original ibtp")
	}

	pd := &pb.Payload{}
	if err := pd.Unmarshal(originalIBTP.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal receipt type ibtp payload: %w", err)
	}

	ct := &pb.Content{}
	contentByte := pd.Content

	var err error
	if pd.Encrypted {
		contentByte, err = e.cryptor.Decrypt(contentByte, originalIBTP.From)
		if err != nil {
			return nil, fmt.Errorf("decrypt ibtp payload content: %w", err)
		}
	}

	if err := ct.Unmarshal(contentByte); err != nil {
		return nil, fmt.Errorf("unmarshal payload content: %w", err)
	}
	pd.Content = contentByte
	originalIBTP.Payload, err = pd.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal payload content: %w", err)
	}

	return e.client.GetReceipt(originalIBTP)
}
