package bxh_lite

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/utils"
	"github.com/sirupsen/logrus"
)

type BxhLite2 struct {
	storage  storage.Storage
	pierID   string
	logger   logrus.FieldLogger
	peerMgr  peermgr.PeerManager
	height   uint64
	headerCh chan *pb.BlockHeader
	pool     *Pool
	ctx      context.Context
	cancel   context.CancelFunc
}

func New2(peerMgr peermgr.PeerManager, storage storage.Storage, logger logrus.FieldLogger, pierId string) (*BxhLite2, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &BxhLite2{
		peerMgr: peerMgr,
		storage: storage,
		logger:  logger,
		pierID:  pierId,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (lite *BxhLite2) Start() error {
	if lite.headerCh == nil {
		lite.headerCh = make(chan *pb.BlockHeader, maxChSize)
	}
	err := lite.peerMgr.RegisterMsgHandler(pb.Message_PIER_SUBSCRIBE_BLOCK_HEADER_ACK, lite.handleMessage)
	if err != nil {
		return fmt.Errorf("handleGetBlockHeader err:%w", err)
	}

	// recover the block height which has latest unfinished interchain tx
	msg := peermgr.Message(pb.Message_PIER_GET_CHAIN_META, true, nil)
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("syncer GetChainMeta marshal err: %w", err)
	}
	meta, err := lite.peerMgr.SendByMultiAddr(data)
	if err != nil {
		return fmt.Errorf("%s, %w", err.Error(), utils.ErrBrokenNetwork)
	}
	chainMeta := &pb.ChainMeta{}
	err = chainMeta.Unmarshal(meta)
	if err != nil {
		lite.logger.WithFields(logrus.Fields{"err": err, "error msg": string(meta)}).Error("unmarshal get chain meta")
		return err
	}

	if chainMeta.Height > lite.height {
		lite.recover(lite.getDemandHeight(), chainMeta.Height)
	}
	lite.pool = NewPool(chainMeta.Height)
	go lite.syncBlock()

	lite.logger.WithFields(logrus.Fields{
		"current_height": lite.height,
		"bitxhub_height": chainMeta.Height,
	}).Info("BitXHub lite started")

	return nil
}

func (lite *BxhLite2) Stop() error {
	lite.cancel()
	lite.logger.Info("BitXHub lite stopped")
	return nil
}

func (lite *BxhLite2) QueryHeader(height uint64) (*pb.BlockHeader, error) {
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
func (lite *BxhLite2) recover(begin, end uint64) {
	lite.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Start BitXHub lite recover")

	requests := utils.GenerateBatchHeaderRequest(begin, end)
	responses := make([]*pb.GetBlockHeadersResponse, len(requests))
	var wg sync.WaitGroup
	wg.Add(len(requests))
	for i, req := range requests {
		go func(i int, req *pb.GetBlockHeaderRequest) {
			defer wg.Done()
			reqData, err := req.Marshal()
			if err != nil {
				lite.logger.Errorf("GetBlockHeaderRequest marshal err :%s", err)
			}
			msg := peermgr.Message(pb.Message_PIER_GET_BLOCK_HEADER, true, reqData)
			data, err := msg.Marshal()
			if err != nil {
				lite.logger.Errorf("GetBlockHeaderRequest message marshal err :%s", err)
			}
			resp, err := lite.peerMgr.SendByMultiAddr(data)
			if err != nil {
				lite.logger.WithFields(logrus.Fields{
					"begin": req.GetBegin(),
					"end":   req.GetEnd(),
					"error": err,
				}).Error("Get block header")
			}
			batchHeaders := &pb.GetBlockHeadersResponse{}
			err = batchHeaders.Unmarshal(resp)
			if err != nil {
				lite.logger.WithFields(logrus.Fields{"err": err, "error msg": string(resp)}).Errorf("GetBlockHeader message from bitxhub unmarshal err :%s", err)
			}
			responses[i] = batchHeaders
		}(i, req)
	}
	wg.Wait()
	for _, headers := range responses {
		for _, h := range headers.BlockHeaders {
			lite.handleBlockHeader(h)
		}
	}
	lite.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Finish BitXHub lite recover")
}

func (lite *BxhLite2) getDemandHeight() uint64 {
	return atomic.LoadUint64(&lite.height) + 1
}

func (lite *BxhLite2) updateHeight() {
	atomic.AddUint64(&lite.height, 1)
}

// Db
func (lite *BxhLite2) persist(h *pb.BlockHeader) error {
	batch := lite.storage.NewBatch()

	data, err := h.Marshal()
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	batch.Put(headerKey(h.Number), data)
	batch.Put(headerHeightKey(), []byte(strconv.FormatUint(lite.height, 10)))

	batch.Commit()

	return nil
}

// getLastHeight gets the current working height of lite
func (lite *BxhLite2) getLastHeight() (uint64, error) {
	v := lite.storage.Get(headerHeightKey())
	if v == nil {
		// if header height is not set, return default 0
		return 0, nil
	}

	return strconv.ParseUint(string(v), 10, 64)
}
