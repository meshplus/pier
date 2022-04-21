package bxh_lite

import (
	"fmt"
	"time"

	"github.com/google/btree"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/sirupsen/logrus"
)

const SubscribeResponse = "Successfully subscribe blockHeader"

func (lite *BxhLite2) handleMessage(s network.Stream, m *pb.Message) {
	handler := func() error {
		switch m.Type {
		case pb.Message_PIER_SUBSCRIBE_BLOCK_HEADER_ACK:
			return lite.handleSubscribeBlockHeader(s, m.Data)
		default:
			lite.logger.WithField("module", "p2p").Errorf("can't handle msg[type: %v]", m.Type)
			return nil
		}
	}

	if err := handler(); err != nil {
		lite.logger.WithFields(logrus.Fields{
			"error": err,
			"type":  m.Type.String(),
		}).Error("Handle message")
	}
}

func (lite *BxhLite2) handleSubscribeBlockHeader(s network.Stream, data []byte) error {
	// successful receive msg, return it to the pangolin p2p module
	if err := s.AsyncSend([]byte(fmt.Sprintf("pierID:%s %s", lite.pierID, SubscribeResponse))); err != nil {
		lite.logger.Errorf("send with stream: %v", err)
		return err
	}
	resp := &pb.Response{}
	if err := resp.Unmarshal(data); err != nil {
		return err
	}
	header := &pb.BlockHeader{}
	err := header.Unmarshal(resp.Data)
	if err != nil {
		return err
	}
	// ensure lite.headerCh is continuous increment
	lite.feedBlockHeader(header)
	return nil
}

type Pool struct {
	headers *btree.BTree
	index   uint64 // record recover finished lite height
	time    time.Time
}

type MyTree struct {
	header *pb.BlockHeader
	height uint64
}

func (m *MyTree) Less(item btree.Item) bool {
	return m.height < (item.(*MyTree)).height
}

func NewPool(index uint64) *Pool {
	return &Pool{
		headers: btree.New(4),
		index:   index,
		time:    time.Now(),
	}
}

func (lite *BxhLite2) feedBlockHeader(header *pb.BlockHeader) {
	lite.pool.headers.ReplaceOrInsert(&MyTree{header: header, height: header.Number})
	if item := lite.pool.headers.Min(); item != nil {
		if item.(*MyTree).height < lite.pool.index+1 {
			lite.pool.headers.DeleteMin()
		}

		if item.(*MyTree).height == lite.pool.index+1 {
			lite.headerCh <- item.(*MyTree).header
			lite.pool.headers.DeleteMin()
			lite.pool.index++
			lite.pool.time = time.Now()
		}

		// By default, if we wait pool index after 10 seconds(maybe missing it), need recover(handleMissing headers)
		if time.Now().Sub(lite.pool.time).Seconds() > 10 {
			lite.recover(lite.pool.index+1, item.(*MyTree).height-1)
			lite.headerCh <- item.(*MyTree).header
			lite.pool.headers.DeleteMin()
			lite.pool.index = item.(*MyTree).height
			lite.pool.time = time.Now()
		}
	}
}
