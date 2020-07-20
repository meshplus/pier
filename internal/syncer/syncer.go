package syncer

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/cbergoon/merkletree"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/lite"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

var _ Syncer = (*WrapperSyncer)(nil)

var logger = log.NewWithModule("syncer")

const maxChSize = 1 << 10

// WrapperSyncer represents the necessary data for sync tx wrappers from bitxhub
type WrapperSyncer struct {
	height   uint64
	agent    agent.Agent
	lite     lite.Lite
	storage  storage.Storage
	wrapperC chan *pb.InterchainTxWrapper
	handler  IBTPHandler

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates instance of WrapperSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(ag agent.Agent, lite lite.Lite, storage storage.Storage) (*WrapperSyncer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &WrapperSyncer{
		wrapperC: make(chan *pb.InterchainTxWrapper, maxChSize),
		agent:    ag,
		lite:     lite,
		storage:  storage,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start implements Syncer
func (syncer *WrapperSyncer) Start() error {
	meta, err := syncer.agent.GetChainMeta()
	if err != nil {
		return fmt.Errorf("get chain meta from bitxhub: %w", err)
	}

	// recover the block height which has latest unfinished interchain tx
	height, err := syncer.getLastHeight()
	if err != nil {
		return fmt.Errorf("get last height: %w", err)
	}
	syncer.height = height

	if meta.Height > height {
		syncer.recover(syncer.getDemandHeight(), meta.Height)
	}

	go syncer.syncInterchainTxWrapper()
	go syncer.listenInterchainTxWrapper()

	logger.WithFields(logrus.Fields{
		"current_height": syncer.height,
		"bitxhub_height": meta.Height,
	}).Info("Syncer started")

	return nil
}

// recover will recover those missing merkle wrapper when pier is down
func (syncer *WrapperSyncer) recover(begin, end uint64) {
	logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Syncer recover")

	ch := make(chan *pb.InterchainTxWrapper, maxChSize)

	if err := syncer.agent.GetInterchainTxWrapper(syncer.ctx, begin, end, ch); err != nil {
		logger.WithFields(logrus.Fields{
			"begin": begin,
			"end":   end,
			"error": err,
		}).Warn("get interchain tx wrapper")
	}

	for w := range ch {
		syncer.handleInterchainTxWrapper(w)
	}
}

// Stop implements Syncer
func (syncer *WrapperSyncer) Stop() error {
	syncer.cancel()

	logger.Info("Syncer stopped")

	return nil
}

// syncInterchainTxWrapper queries to bitxhub and syncs confirmed interchain txs
// whose destination is the same as pierID.
// Note: only interchain txs generated after the connection to bitxhub
// being established will be sent to syncer
func (syncer *WrapperSyncer) syncInterchainTxWrapper() {
	loop := func(ch <-chan *pb.InterchainTxWrapper) {
		for {
			select {
			case wrapper, ok := <-ch:
				if !ok {
					logger.Warn("Unexpected closed channel while syncing interchain tx wrapper")
					return
				}

				syncer.wrapperC <- wrapper
			case <-syncer.ctx.Done():
				return
			}
		}

	}

	for {
		select {
		case <-syncer.ctx.Done():
			return
		default:
			ch := syncer.getWrapperChannel()

			err := retry.Retry(func(attempt uint) error {
				chainMeta, err := syncer.agent.GetChainMeta()
				if err != nil {
					logger.WithField("error", err).Error("Get chain meta")
					return err
				}

				if chainMeta.Height > syncer.height {
					syncer.recover(syncer.getDemandHeight(), chainMeta.Height)
				}

				return nil
			}, strategy.Wait(1*time.Second))

			if err != nil {
				logger.Panic(err)
			}

			loop(ch)
		}
	}
}

// getWrapperChannel gets a syncing merkle wrapper channel
func (syncer *WrapperSyncer) getWrapperChannel() chan *pb.InterchainTxWrapper {
	ch := make(chan *pb.InterchainTxWrapper, maxChSize)

	if err := retry.Retry(func(attempt uint) error {
		if err := syncer.agent.SyncInterchainTxWrapper(syncer.ctx, ch); err != nil {
			return err
		}

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	return ch
}

// listenInterchainTxWrapper listen on the wrapper channel for handling
func (syncer *WrapperSyncer) listenInterchainTxWrapper() {
	for {
		select {
		case w := <-syncer.wrapperC:
			if w.Height < syncer.getDemandHeight() {
				logger.WithField("height", w.Height).Warn("Discard wrong wrapper")
				continue
			}

			if w.Height > syncer.getDemandHeight() {
				logger.WithFields(logrus.Fields{
					"begin": syncer.height,
					"end":   w.Height,
				}).Info("Get interchain tx wrapper")

				ch := make(chan *pb.InterchainTxWrapper, maxChSize)
				if err := syncer.agent.GetInterchainTxWrapper(syncer.ctx, syncer.getDemandHeight(), w.Height, ch); err != nil {
					logger.WithFields(logrus.Fields{
						"begin": syncer.height,
						"end":   w.Height,
						"error": err,
					}).Warn("Get interchain tx wrapper")
				}

				for w := range ch {
					syncer.handleInterchainTxWrapper(w)
				}
				continue
			}

			syncer.handleInterchainTxWrapper(w)
		case <-syncer.ctx.Done():
			return
		}
	}
}

// handleInterchainTxWrapper is the handler for interchain tx wrapper
func (syncer *WrapperSyncer) handleInterchainTxWrapper(w *pb.InterchainTxWrapper) {
	if w == nil {
		logger.WithField("height", syncer.height).Error("empty interchain tx wrapper")
		return
	}

	if w.Height < syncer.getDemandHeight() {
		logger.Warn("wrong height")
		return
	}

	if ok, err := syncer.verifyWrapper(w); !ok {
		logger.WithFields(logrus.Fields{
			"height": w.Height,
			"error":  err,
		}).Warn("Invalid wrapper")
		return
	}

	for _, tx := range w.Transactions {
		ibtp, err := tx.GetIBTP()
		if err != nil {
			logger.Errorf("Get ibtp from tx: %s", err.Error())
			continue
		}
		syncer.handler(ibtp)
	}

	logger.WithFields(logrus.Fields{
		"height": w.Height,
		"count":  len(w.Transactions),
	}).Info("Persist interchain tx wrapper")

	if err := syncer.persist(w); err != nil {
		logger.WithFields(logrus.Fields{
			"height": w.Height,
			"error":  err,
		}).Error("Persist interchain tx wrapper")
	}

	syncer.updateHeight()
}

func (syncer *WrapperSyncer) RegisterIBTPHandler(handler IBTPHandler) error {
	if handler == nil {
		return fmt.Errorf("register ibtp handler: empty handler")
	}

	syncer.handler = handler
	return nil
}

// verifyWrapper verifies the basic of merkle wrapper from bitxhub
func (syncer *WrapperSyncer) verifyWrapper(w *pb.InterchainTxWrapper) (bool, error) {
	if w.Height != syncer.getDemandHeight() {
		return false, fmt.Errorf("wrong height of wrapper from bitxhub")
	}

	if w.Height == 1 || w.TransactionHashes == nil {
		return true, nil
	}

	if len(w.TransactionHashes) != len(w.Transactions) {
		return false, fmt.Errorf("wrong size of interchain txs from bitxhub, hashes :%d, txs: %d", len(w.TransactionHashes), len(w.Transactions))
	}

	// validate if l2roots are correct
	l2RootHashes := make([]merkletree.Content, 0, len(w.L2Roots))
	for _, root := range w.L2Roots {
		l2RootHashes = append(l2RootHashes, pb.TransactionHash(root.Bytes()))
	}
	l1Tree, err := merkletree.NewTree(l2RootHashes)
	if err != nil {
		return false, fmt.Errorf("init l1 merkle tree: %w", err)
	}

	var header *pb.BlockHeader
	if err := retry.Retry(func(attempt uint) error {
		header, err = syncer.lite.QueryHeader(w.Height)
		if err != nil {
			if err == leveldb.ErrNotFound {
				return fmt.Errorf("query block header :%w", err)
			}
			panic(err)
		}

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	// verify tx root
	if types.Bytes2Hash(l1Tree.MerkleRoot()) != header.TxRoot {
		return false, fmt.Errorf("tx wrapper is wrong")
	}

	// validate if the txs is committed in bitxhub
	if len(w.Transactions) == 0 {
		return true, nil
	}

	hashes := make([]merkletree.Content, 0, len(w.TransactionHashes))
	existM := make(map[string]bool)
	for _, hash := range w.TransactionHashes {
		hashes = append(hashes, pb.TransactionHash(hash.Bytes()))
		existM[hash.String()] = true
	}

	tree, err := merkletree.NewTree(hashes)
	if err != nil {
		return false, fmt.Errorf("init merkle tree: %w", err)
	}

	l2root := tree.MerkleRoot()
	correctRoot := false
	for _, rootHash := range w.L2Roots {
		if bytes.Equal(rootHash.Bytes(), l2root) {
			correctRoot = true
			break
		}
	}
	if !correctRoot {
		return false, fmt.Errorf("incorrect trx hashes!")
	}

	// verify if every interchain tx is valid
	for _, tx := range w.Transactions {
		if existM[tx.TransactionHash.String()] {
			// TODO: how to deal with malicious tx found
			continue
		}
	}

	return true, nil
}

// getLastHeight gets the current working height of Syncer
func (syncer *WrapperSyncer) getLastHeight() (uint64, error) {
	v, err := syncer.storage.Get(syncHeightKey())
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}

		return 0, fmt.Errorf("get syncer height %w", err)
	}

	return strconv.ParseUint(string(v), 10, 64)
}

func syncHeightKey() []byte {
	return []byte("sync-height")
}

func (syncer *WrapperSyncer) getDemandHeight() uint64 {
	return atomic.LoadUint64(&syncer.height) + 1
}

// updateHeight updates sync height and
func (syncer *WrapperSyncer) updateHeight() {
	atomic.AddUint64(&syncer.height, 1)
}
