package syncer

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

var _ Syncer = (*MerkleSyncer)(nil)

var logger = log.NewWithModule("syncer")

const wrapperCNumber = 1 << 10

// MerkleSyncer represents the necessary data for sync block wrappers from bitxhub
type MerkleSyncer struct {
	height     uint64
	agent      agent.Agent
	quorum     uint64
	validators []types.Address
	storage    storage.Storage
	wrapperC   chan *pb.MerkleWrapper

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates instance of MerkleSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(ag agent.Agent, quorum uint64, validators []types.Address, storage storage.Storage) (*MerkleSyncer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &MerkleSyncer{
		wrapperC:   make(chan *pb.MerkleWrapper, wrapperCNumber),
		agent:      ag,
		quorum:     quorum,
		validators: validators,
		storage:    storage,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start implements Syncer
func (syncer *MerkleSyncer) Start() error {
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

	go syncer.syncMerkleWrapper()
	go syncer.listenMerkleWrapper()

	logger.WithFields(logrus.Fields{
		"current_height": syncer.height,
		"bitxhub_height": meta.Height,
	}).Info("Syncer started")

	return nil
}

// recover will recover those missing merkle wrapper when pier is down
func (syncer *MerkleSyncer) recover(begin, end uint64) {
	logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Syncer recover")

	ch, err := syncer.agent.GetMerkleWrapper(begin, end)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"begin": begin,
			"end":   end,
			"error": err,
		}).Warn("get merkle wrapper")
	}

	for w := range ch {
		syncer.handleMerkleWrapper(w)
	}
}

// Stop implements Syncer
func (syncer *MerkleSyncer) Stop() error {
	syncer.cancel()

	logger.Info("Syncer stopped")

	return nil
}

func (syncer *MerkleSyncer) QueryWrapper(height uint64) (*pb.MerkleWrapper, error) {
	// get wrapper from storage
	v, err := syncer.storage.Get(model.WrapperKey(height))
	if err != nil {
		return nil, fmt.Errorf("get wrapper by key %d: %w", height, err)
	}

	w := &pb.MerkleWrapper{}
	if err := w.Unmarshal(v); err != nil {
		return nil, fmt.Errorf("unmarshal wrapper: %w", err)
	}

	return w, nil
}

// syncMerkleWrapper queries to bitxhub and syncs confirmed block
// Note: only block wrappers generated after the connection to bitxhub
// being established will be sent to syncer
func (syncer *MerkleSyncer) syncMerkleWrapper() {
	loop := func(ch <-chan *pb.MerkleWrapper) {
		for {
			select {
			case wrapper, ok := <-ch:
				if !ok {
					logger.Warn("Unexpected closed channel while syncing merkle wrapper")
					return
				}

				syncer.wrapperC <- wrapper
			case <-syncer.ctx.Done():
				return
			}
		}

	}

	for {
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

// getWrapperChannel gets a syncing merkle wrapper channel
func (syncer *MerkleSyncer) getWrapperChannel() chan *pb.MerkleWrapper {
	var ch chan *pb.MerkleWrapper

	if err := retry.Retry(func(attempt uint) error {
		c, err := syncer.agent.SyncMerkleWrapper(syncer.ctx)
		if err != nil {
			return err
		}

		ch = c

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	return ch
}

// listenMerkleWrapper listen on the wrapper channel for handling
func (syncer *MerkleSyncer) listenMerkleWrapper() {
	for {
		select {
		case w := <-syncer.wrapperC:
			if w.BlockHeader.Number < syncer.getDemandHeight() {
				logger.WithField("height", w.BlockHeader.Number).Warn("Discard wrong wrapper")
				continue
			}

			if w.BlockHeader.Number > syncer.getDemandHeight() {
				logger.WithFields(logrus.Fields{
					"begin": syncer.height,
					"end":   w.BlockHeader.Number,
				}).Info("Get merkle wrapper")

				ch, err := syncer.agent.GetMerkleWrapper(syncer.getDemandHeight(), w.BlockHeader.Number)
				if err != nil {
					logger.WithFields(logrus.Fields{
						"begin": syncer.height,
						"end":   w.BlockHeader.Number,
						"error": err,
					}).Warn("get merkle wrapper")
				}

				for w := range ch {
					syncer.handleMerkleWrapper(w)
				}
				continue
			}

			syncer.handleMerkleWrapper(w)
		case <-syncer.ctx.Done():
			return
		}
	}
}

// handleMerkleWrapper is the handler for merkle wrapper
func (syncer *MerkleSyncer) handleMerkleWrapper(w *pb.MerkleWrapper) {
	if w == nil {
		logger.WithField("height", syncer.height).Error("empty merkle wrapper")
		return
	}

	if w.BlockHeader.Number < syncer.getDemandHeight() {
		return
	}

	if ok, err := syncer.verifyWrapper(w); !ok {
		logger.WithFields(logrus.Fields{
			"height": w.BlockHeader.Number,
			"error":  err,
		}).Warn("Invalid wrapper")
		return
	}

	logger.WithFields(logrus.Fields{
		"height": w.BlockHeader.Number,
		"count":  len(w.Transactions),
	}).Info("Persist merkle wrapper")

	if err := syncer.persist(w); err != nil {
		logger.WithFields(logrus.Fields{
			"height": w.BlockHeader.Number,
			"error":  err,
		}).Error("Persist merkle wrapper")
	}

	syncer.updateHeight()
}

// verifyWrapper verifies the basic of merkle wrapper from bitxhub
func (syncer *MerkleSyncer) verifyWrapper(w *pb.MerkleWrapper) (bool, error) {
	if w == nil || w.BlockHeader == nil {
		return false, fmt.Errorf("empty wrapper or block header")
	}

	// validate if the wrapper is from bitxhub
	if w.BlockHeader.Number != syncer.getDemandHeight() {
		return false, fmt.Errorf("wrong height of wrapper from bitxhub")
	}

	// todo: wait for bitxhub to provide signatures for wrapper, now verify always return true
	if uint64(len(w.Signatures)) <= syncer.quorum {
		return true, fmt.Errorf("%d signatures not meet the requirement of quorum %d", len(w.Signatures), syncer.quorum)
	}

	count := uint64(0)
	var wg sync.WaitGroup
	wg.Add(len(syncer.validators))

	for _, validator := range syncer.validators {
		sign, ok := w.Signatures[validator.String()]
		if ok {
			go func(vlt types.Address, sign []byte) {
				if isValid, _ := asym.Verify(asym.ECDSASecp256r1, sign, w.SignHash().Bytes(), vlt); isValid {
					atomic.AddUint64(&count, 1)
				}
			}(validator, sign)
		}
		wg.Done()
	}

	wg.Wait()
	if count <= syncer.quorum {
		return true, fmt.Errorf("invalid signature")
	}

	return true, nil
}

// getLastHeight gets the current working height of Syncer
func (syncer *MerkleSyncer) getLastHeight() (uint64, error) {
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

func (syncer *MerkleSyncer) getDemandHeight() uint64 {
	return atomic.LoadUint64(&syncer.height) + 1
}

// updateHeight updates sync height and
func (syncer *MerkleSyncer) updateHeight() {
	atomic.AddUint64(&syncer.height, 1)
}
