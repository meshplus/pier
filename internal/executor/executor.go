package executor

import (
	"context"
	"fmt"

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
	client          plugins.Client // the client to interact with appchain
	storage         storage.Storage
	id              string            // appchain id
	executeMeta     map[string]uint64 // pier execute crosschain ibtp index map
	callbackMeta    map[string]uint64 // pier execute callback index map
	srcRollbackMeta map[string]uint64 // pier execute rollback index map as src chain pier
	dstRollbackMeta map[string]uint64 // pier execute rollback index map as dst chain pier
	cryptor         txcrypto.Cryptor
	ctx             context.Context
	cancel          context.CancelFunc
}

// New creates new instance of Executor. agent is for interacting with counterpart chain
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func New(client plugins.Client, pierID string, storage storage.Storage, cryptor txcrypto.Cryptor) (*ChannelExecutor, error) {
	execMeta, err := client.GetInMeta()
	if err != nil {
		return nil, fmt.Errorf("get in executeMeta: %w", err)
	}
	if execMeta == nil {
		execMeta = make(map[string]uint64)
	}

	callbackMeta, err := client.GetCallbackMeta() // callback that broker contract has executed
	if err != nil {
		return nil, fmt.Errorf("get callback executeMeta: %w", err)
	}
	if callbackMeta == nil {
		callbackMeta = make(map[string]uint64)
	}

	srcRollbackMeta, err := client.GetSrcRollbackMeta() // rollback that src chain broker contract has executed
	if err != nil {
		return nil, fmt.Errorf("get src rollback executeMeta: %w", err)
	}
	if srcRollbackMeta == nil {
		srcRollbackMeta = make(map[string]uint64)
	}

	dsrRollbackMeta, err := client.GetDstRollbackMeta() // callback that broker contract has executed
	if err != nil {
		return nil, fmt.Errorf("get dst rollback executeMeta: %w", err)
	}
	if dsrRollbackMeta == nil {
		dsrRollbackMeta = make(map[string]uint64)
	}

	logger.WithFields(logrus.Fields{
		"inMeta(execMeta)": execMeta,
		"callbackMeta":     callbackMeta,
		"srcRollbackMeta":  srcRollbackMeta,
		"dstRollbackMeta":  dsrRollbackMeta,
	}).Info("Get meta from broker")

	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:          client,
		ctx:             ctx,
		cancel:          cancel,
		storage:         storage,
		id:              pierID,
		executeMeta:     execMeta,
		callbackMeta:    callbackMeta,
		srcRollbackMeta: srcRollbackMeta,
		dstRollbackMeta: dsrRollbackMeta,
		cryptor:         cryptor,
	}, nil
}

// Start implements Executor
func (e *ChannelExecutor) Start() error {
	logger.Info("Executor started")

	for from, idx := range e.executeMeta {
		logger.WithFields(logrus.Fields{
			"from":  from,
			"index": idx,
		}).Info("Execution index in appchain")
	}

	return nil
}

// Stop implements Executor
func (e *ChannelExecutor) Stop() error {
	e.cancel()

	logger.Info("Executor stopped")

	return nil
}

func (e *ChannelExecutor) QueryLatestMeta() map[string]uint64 {
	return e.executeMeta
}

func (e *ChannelExecutor) QueryDstRollbackMeta() map[string]uint64 {
	return e.dstRollbackMeta
}

// getReceipt only generates one receipt given source chain id and interchain tx index
func (e *ChannelExecutor) QueryReceipt(from string, idx uint64, originalIBTP *pb.IBTP) (*pb.IBTP, error) {
	ret, err := e.client.GetInMessage(from, idx)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt message from appchain: %w", err)
	}

	return e.generateCallback(originalIBTP, ret)
}
