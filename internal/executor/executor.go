package executor

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/model"
	"github.com/meshplus/pier/pkg/plugins/client"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

var logger = log.NewWithModule("executor")

// ChannelExecutor represents the necessary data for executing interchain txs in appchain
type ChannelExecutor struct {
	client            client.Client // the client to interact with appchain
	agent             agent.Agent   // the agent to interact with bitxhub
	storage           storage.Storage
	id                string            // appchain id
	height            uint64            // current bitxhub block height executor is working on
	executeMeta       map[string]uint64 // pier execute crosschain ibtp index map
	callbackMeta      map[string]uint64 // pier execute callback index map
	sourceReceiptMeta map[string]uint64 // bitxhub receives receipts from the pier index map
	cryptor           txcrypto.Cryptor

	ctx    context.Context
	cancel context.CancelFunc
}

// NewChannelExecutor creates new instance of Executor. agent is for interacting with bitxhub
// client is for interacting with appchain, meta is for recording interchain tx meta information
// and ds is for persisting some runtime messages
func NewChannelExecutor(agent agent.Agent, client client.Client, meta *rpcx.Appchain, storage storage.Storage, cryptor txcrypto.Cryptor) (*ChannelExecutor, error) {
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

	if meta.ReceiptCounter == nil {
		meta.ReceiptCounter = make(map[string]uint64)
	}

	if meta.SourceReceiptCounter == nil {
		meta.SourceReceiptCounter = make(map[string]uint64)
	}

	height, err := getExecBlockHeight(storage)
	if err != nil {
		return nil, fmt.Errorf("get executor height: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ChannelExecutor{
		client:            client,
		agent:             agent,
		ctx:               ctx,
		cancel:            cancel,
		storage:           storage,
		id:                meta.ID,
		height:            height,
		executeMeta:       execMeta,
		callbackMeta:      callbackMeta,
		sourceReceiptMeta: meta.SourceReceiptCounter,
		cryptor:           cryptor,
	}, nil
}

// Start implements Executor
func (e *ChannelExecutor) Start() error {
	logger.WithFields(logrus.Fields{
		"execute_height": e.height,
	}).Info("Executor started")

	for from, idx := range e.executeMeta {
		logger.WithFields(logrus.Fields{
			"from":  from,
			"index": idx,
		}).Info("Execution index in appchain")
	}

	for from, idx := range e.sourceReceiptMeta {
		logger.WithFields(logrus.Fields{
			"from":  from,
			"index": idx,
		}).Info("Receipt index in bitxhub")
	}

	if err := e.recover(); err != nil {
		return fmt.Errorf("recover: %w", err)
	}

	go e.run()

	return nil
}

// Stop implements Executor
func (e *ChannelExecutor) Stop() error {
	e.cancel()

	logger.Info("Executor stopped")

	return e.client.Stop()
}

// run starts a long run goroutine which is responsible for respectively check
// if next wanted new wrapper is synced from bitxhub
func (e *ChannelExecutor) run() {
	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			data, err := e.storage.Get(model.WrapperKey(e.getDemandHeight()))
			if err != nil {
				if err == leveldb.ErrNotFound {
					continue
				}

				logger.WithFields(logrus.Fields{
					"height": e.getDemandHeight(),
					"error":  err,
				}).Error("Get merkle wrapper from storage")
				continue
			}

			w := &pb.InterchainTxWrapper{}
			if err := w.Unmarshal(data); err != nil {
				logger.WithFields(logrus.Fields{
					"height": e.getDemandHeight(),
					"error":  err,
				}).Error("Unmarshal merkle wrapper")
				continue
			}

			e.applyInterchainTxWrapper(w)
		case <-e.ctx.Done():
			return
		}
	}
}

// getDemandHeight get next wanted wrapper height
func (e *ChannelExecutor) getDemandHeight() uint64 {
	return atomic.LoadUint64(&e.height) + 1
}
