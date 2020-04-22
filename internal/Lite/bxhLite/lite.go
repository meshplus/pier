package bxhLite

import (
	"context"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/Lite"
	"github.com/meshplus/pier/internal/agent"
	"github.com/sirupsen/logrus"
)

var _ Lite.Lite = (*BxhLite)(nil)

var logger = log.NewWithModule("bxhLite")

const maxSize = 1 << 10

// BxhLite represents the necessary data for sync block wrappers from bitxhub
type BxhLite struct {
	height     uint64
	chainID    string
	agent      agent.Agent
	quorum     uint64
	validators []types.Address
	storage    storage.Storage
	recvCh     chan *pb.IBTP
	wrapperC   chan *pb.MerkleWrapper

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates instance of BxhLite given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(ag agent.Agent, quorum uint64, chainID string, validators []types.Address, storage storage.Storage) (*BxhLite, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &BxhLite{
		wrapperC:   make(chan *pb.MerkleWrapper, maxSize),
		recvCh:     make(chan *pb.IBTP, maxSize),
		agent:      ag,
		quorum:     quorum,
		chainID:    chainID,
		validators: validators,
		storage:    storage,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start implements Syncer
func (lite *BxhLite) Start() error {
	meta, err := lite.agent.GetChainMeta()
	if err != nil {
		return fmt.Errorf("get chain meta from bitxhub: %w", err)
	}

	// recover the block height which has latest unfinished interchain tx
	height, err := lite.getLastHeight()
	if err != nil {
		return fmt.Errorf("get last height: %w", err)
	}
	lite.height = height

	if meta.Height > height {
		lite.recover(lite.getDemandHeight(), meta.Height)
	}

	go lite.syncMerkleWrapper()
	go lite.listenMerkleWrapper()

	logger.WithFields(logrus.Fields{
		"current_height": lite.height,
		"bitxhub_height": meta.Height,
	}).Info("Syncer started")

	return nil
}

// recover will recover those missing merkle wrapper when pier is down
func (lite *BxhLite) recover(begin, end uint64) {
	logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Syncer recover")

	ch, err := lite.agent.GetMerkleWrapper(begin, end)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"begin": begin,
			"end":   end,
			"error": err,
		}).Warn("get merkle wrapper")
	}

	for w := range ch {
		lite.handleMerkleWrapper(w)
	}
}

// Stop implements Syncer
func (lite *BxhLite) Stop() error {
	lite.cancel()

	logger.Info("Syncer stopped")

	return nil
}

// syncMerkleWrapper queries to bitxhub and syncs confirmed block
// Note: only block wrappers generated after the connection to bitxhub
// being established will be sent to bxhLite
func (lite *BxhLite) syncMerkleWrapper() {
	loop := func(ch <-chan *pb.MerkleWrapper) {
		for {
			select {
			case wrapper, ok := <-ch:
				if !ok {
					logger.Warn("Unexpected closed channel while syncing merkle wrapper")
					return
				}

				lite.wrapperC <- wrapper
			case <-lite.ctx.Done():
				return
			}
		}

	}

	for {
		ch := lite.getWrapperChannel()

		err := retry.Retry(func(attempt uint) error {
			chainMeta, err := lite.agent.GetChainMeta()
			if err != nil {
				logger.WithField("error", err).Error("Get chain meta")
				return err
			}

			if chainMeta.Height > lite.height {
				lite.recover(lite.getDemandHeight(), chainMeta.Height)
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
func (lite *BxhLite) getWrapperChannel() chan *pb.MerkleWrapper {
	var ch chan *pb.MerkleWrapper

	if err := retry.Retry(func(attempt uint) error {
		c, err := lite.agent.SyncMerkleWrapper(lite.ctx)
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
func (lite *BxhLite) listenMerkleWrapper() {
	for {
		select {
		case w := <-lite.wrapperC:
			if w.BlockHeader.Number < lite.getDemandHeight() {
				logger.WithField("height", w.BlockHeader.Number).Warn("Discard wrong wrapper")
				continue
			}

			if w.BlockHeader.Number > lite.getDemandHeight() {
				logger.WithFields(logrus.Fields{
					"begin": lite.height,
					"end":   w.BlockHeader.Number,
				}).Info("Get merkle wrapper")

				ch, err := lite.agent.GetMerkleWrapper(lite.getDemandHeight(), w.BlockHeader.Number)
				if err != nil {
					logger.WithFields(logrus.Fields{
						"begin": lite.height,
						"end":   w.BlockHeader.Number,
						"error": err,
					}).Warn("get merkle wrapper")
				}

				for w := range ch {
					lite.handleMerkleWrapper(w)
				}
				continue
			}

			lite.handleMerkleWrapper(w)
		case <-lite.ctx.Done():
			return
		}
	}
}

// handleMerkleWrapper is the handler for merkle wrapper
func (lite *BxhLite) handleMerkleWrapper(w *pb.MerkleWrapper) {
	if w == nil {
		logger.WithField("height", lite.height).Error("empty merkle wrapper")
		return
	}

	if w.BlockHeader.Number < lite.getDemandHeight() {
		return
	}

	if ok, err := lite.verifyWrapper(w); !ok {
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

	if err := lite.persist(w); err != nil {
		logger.WithFields(logrus.Fields{
			"height": w.BlockHeader.Number,
			"error":  err,
		}).Error("Persist merkle wrapper")
	}

	lite.applyMerkleWrapper(w)
	lite.updateHeight()
}

func (lite *BxhLite) RecvIBTP() chan *pb.IBTP {
	return lite.recvCh
}

func (lite *BxhLite) SendIBTP(ibtp *pb.IBTP) *pb.Receipt {
	entry := logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"to":    types.String2Address(ibtp.To).ShortString(),
		"id":    ibtp.ID(),
	})

	var (
		receipt = &pb.Receipt{}
		err     error
	)

	send := func() error {
		receipt, err = lite.agent.SendIBTP(ibtp)
		if err != nil {
			return err
		}

		entry.WithFields(logrus.Fields{
			"hash": receipt.TxHash.Hex(),
		}).Info("Send ibtp")

		return nil
	}

	if err := retry.Retry(func(attempt uint) error {
		if err := send(); err != nil {
			entry.WithFields(logrus.Fields{
				"error": err,
			}).Error("Send IBTP")

			return err
		}

		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		panic(err.Error())
	}

	return receipt
}

func (lite *BxhLite) applyMerkleWrapper(wrapper *pb.MerkleWrapper) {
	ibtps, err := lite.verifyTxs(wrapper)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Panic("Invalid wrapper")

		return
	}

	defer logger.WithFields(logrus.Fields{
		"height": wrapper.BlockHeader.Number,
	}).Info("Finish merkle wrapper")

	for _, ibtp := range ibtps {
		lite.recvCh <- ibtp
	}
}
