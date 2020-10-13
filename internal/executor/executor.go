package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("executor")

// ChannelExecutor represents the necessary data for executing interchain txs in appchain
type ChannelExecutor struct {
	client       plugins.Client // the client to interact with appchain
	storage      storage.Storage
	id           string   // appchain id
	executeMeta  sync.Map // pier execute crosschain ibtp index map
	callbackMeta sync.Map // pier execute callback index map
	cryptor      txcrypto.Cryptor
	ctx          context.Context
	cancel       context.CancelFunc
}

// New creates new instance of Executor. agent is for interacting with counterpart chain
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func New(client plugins.Client, pierID string, storage storage.Storage, cryptor txcrypto.Cryptor) (*ChannelExecutor, error) {
	execMeta, err := client.GetInMeta()
	if err != nil {
		return nil, fmt.Errorf("get in executeMeta: %w", err)
	}

	executeMeta := sync.Map{}
	if execMeta != nil {
		for id, count := range execMeta {
			executeMeta.Store(id, count)
		}
	}

	callMeta, err := client.GetCallbackMeta() // callback that broker contract has executed
	if err != nil {
		return nil, fmt.Errorf("get callback executeMeta: %w", err)
	}

	callbackMeta := sync.Map{}
	if callMeta != nil {
		for id, count := range callMeta {
			callbackMeta.Store(id, count)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:       client,
		ctx:          ctx,
		cancel:       cancel,
		storage:      storage,
		id:           pierID,
		executeMeta:  executeMeta,
		callbackMeta: callbackMeta,
		cryptor:      cryptor,
	}, nil
}

// Start implements Executor
func (e *ChannelExecutor) Start() error {
	logger.Info("Executor started")

	e.executeMeta.Range(func(id, idx interface{}) bool {
		logger.WithFields(logrus.Fields{
			"from":  id,
			"index": idx,
		}).Info("Execution index in appchain")
		return true
	})
	return nil
}

// Stop implements Executor
func (e *ChannelExecutor) Stop() error {
	e.cancel()

	logger.Info("Executor stopped")

	return nil
}

func (e *ChannelExecutor) QueryLatestMeta() *sync.Map {
	return &e.executeMeta
}

func (e *ChannelExecutor) QueryLatestCallbackMeta() *sync.Map {
	return &e.callbackMeta
}

// getReceipt only generates one receipt given source chain id and interchain tx index
func (e *ChannelExecutor) QueryReceipt(from string, idx uint64, originalIBTP *pb.IBTP) (*pb.IBTP, error) {
	ret, err := e.client.GetInMessage(from, idx)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt message from appchain: %w", err)
	}

	return e.generateCallback(originalIBTP, ret)
}
