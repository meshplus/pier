package syncer

import (
	"fmt"
	"time"

	"github.com/google/btree"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/sirupsen/logrus"
)

const SubscribeResponse = "Successfully subscribe interchainTxWrappers"

func (syncer *WrapperSyncer2) handleMessage(s network.Stream, m *pb.Message) {
	handler := func() error {
		switch m.Type {
		case pb.Message_PIER_SUBSCRIBE_INTERCHAIN_TX_WRAPPERS_ACK:
			return syncer.handleSubscribeInterchainTxWrappers(s, m.Data)
		default:
			syncer.logger.WithField("module", "p2p").Errorf("can't handle msg[type: %v]", m.Type)
			return nil
		}
	}

	if err := handler(); err != nil {
		syncer.logger.WithFields(logrus.Fields{
			"error": err,
			"type":  m.Type.String(),
		}).Error("Handle message")
	}
}

func (syncer *WrapperSyncer2) handleSubscribeInterchainTxWrappers(stream network.Stream, data []byte) error {
	// successful receive msg, return it to the pangolin p2p module
	if err := stream.AsyncSend([]byte(fmt.Sprintf("pierID:%s %s", syncer.pierID, SubscribeResponse))); err != nil {
		syncer.logger.Errorf("send with stream: %v", err)
		return err
	}
	resp := &pb.Response{}
	if err := resp.Unmarshal(data); err != nil {
		return err
	}
	wrapers := &pb.InterchainTxWrappers{}
	err := wrapers.Unmarshal(resp.Data)
	if err != nil {
		return err
	}
	syncer.feedInterchainWrapers(wrapers)
	return nil
}

type Pool struct {
	multiWrappers *btree.BTree
	index         uint64 // record recover finished syncer height
	time          time.Time
}

type MyTree struct {
	wrapers *pb.InterchainTxWrappers
	height  uint64
}

func (m *MyTree) Less(item btree.Item) bool {
	return m.height < (item.(*MyTree)).height
}

func NewPool(index uint64) *Pool {
	return &Pool{
		multiWrappers: btree.New(4),
		index:         index,
		time:          time.Now(),
	}
}

func (syncer *WrapperSyncer2) feedInterchainWrapers(wappers *pb.InterchainTxWrappers) {
	syncer.pool.multiWrappers.ReplaceOrInsert(&MyTree{wrapers: wappers, height: wappers.InterchainTxWrappers[0].GetHeight()})
	if item := syncer.pool.multiWrappers.Min(); item != nil {
		if item.(*MyTree).height < syncer.pool.index+1 {
			syncer.pool.multiWrappers.DeleteMin()
		} else if item.(*MyTree).height == syncer.pool.index+1 {
			syncer.wrappersC <- item.(*MyTree).wrapers
			syncer.pool.multiWrappers.DeleteMin()
			syncer.pool.index++
			syncer.pool.time = time.Now()
		} else if time.Now().Sub(syncer.pool.time).Seconds() > 60 {
			// By default, if we wait pool index after 60 seconds(maybe missing it), need recover(handleMissing multiWrappers)
			syncer.recover(syncer.pool.index+1, item.(*MyTree).height-1)
			syncer.wrappersC <- item.(*MyTree).wrapers
			syncer.pool.multiWrappers.DeleteMin()
			syncer.pool.index = item.(*MyTree).height
			syncer.pool.time = time.Now()
		}
	}
}
