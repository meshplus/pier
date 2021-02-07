package bxh_lite

import (
	"context"
	"fmt"

	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/sirupsen/logrus"
)

const maxChSize = 1024

type BxhLite struct {
	client  rpcx.Client
	storage storage.Storage
	logger  logrus.FieldLogger
	height  uint64
	ctx     context.Context
	cancel  context.CancelFunc
}

func New(client rpcx.Client, storage storage.Storage, logger logrus.FieldLogger) (*BxhLite, error) {
	return &BxhLite{
		client:  client,
		storage: storage,
		logger:  logger,
	}, nil
}

func (lite *BxhLite) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	lite.ctx = ctx
	lite.cancel = cancel

	meta, err := lite.client.GetChainMeta()
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

	go lite.syncBlock()

	lite.logger.WithFields(logrus.Fields{
		"current_height": lite.height,
		"bitxhub_height": meta.Height,
	}).Info("BitXHub lite started")

	return nil
}

func (lite *BxhLite) Stop() error {
	lite.cancel()

	lite.logger.Info("BitXHub lite stopped")
	return nil
}

func (lite *BxhLite) QueryHeader(height uint64) (*pb.BlockHeader, error) {
	v := lite.storage.Get(headerKey(height))
	if v == nil {
		return nil, fmt.Errorf("header at %d not found", height)
	}

	header := &pb.BlockHeader{}
	if err := header.Unmarshal(v); err != nil {
		return nil, err
	}

	return header, nil
}

// recover will recover those missing merkle wrapper when pier is down
func (lite *BxhLite) recover(begin, end uint64) {
	lite.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("BitXHub lite recover")

	headerCh := make(chan *pb.BlockHeader, maxChSize)
	if err := lite.client.GetBlockHeader(lite.ctx, begin, end, headerCh); err != nil {
		lite.logger.WithFields(logrus.Fields{
			"begin": begin,
			"end":   end,
			"error": err,
		}).Warn("Get block header")
	}

	for h := range headerCh {
		lite.handleBlockHeader(h)
	}
}
