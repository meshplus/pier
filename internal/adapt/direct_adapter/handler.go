package direct_adapter

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
)

type Pool struct {
	ibtps *btree.BTree
	lock  *sync.Mutex
	time  time.Time
}

type MyTree struct {
	ibtp  *pb.IBTP
	index uint64
}

func (m *MyTree) Less(item btree.Item) bool {
	return m.index < (item.(*MyTree)).index
}

func (d *DirectAdapter) handleGetIBTPMessage(stream network.Stream, msg *pb.Message) {
	ibtpID := string(peermgr.DataToPayload(msg).Data)
	var isReq bool
	if msg.Type == pb.Message_IBTP_GET {
		isReq = true
	}
	// if msg is receipt, isReq is false
	ibtp, err := d.appchainadapt.QueryIBTP(ibtpID, isReq)
	if err != nil {
		d.logger.Error("Get wrong ibtp id")
		return
	}

	data, err := ibtp.Marshal()
	if err != nil {
		return
	}

	retMsg := peermgr.Message(pb.Message_ACK, true, data)

	err = d.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		d.logger.Error(err)
		return
	}
}

func NewPool() *Pool {
	return &Pool{
		ibtps: btree.New(4),
		lock:  &sync.Mutex{},
		time:  time.Now(),
	}
}

func (d *DirectAdapter) handleSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	d.gopool.Add()
	go func() {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			d.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		servicePair := ibtp.From + ibtp.To + ibtp.Category().String()
		act, loaded := d.ibtps.LoadOrStore(servicePair, NewPool())
		pool := act.(*Pool)

		if !loaded {
			go func(pool *Pool, ibtp *pb.IBTP) {
				defer func() {
					if e := recover(); e != nil {
						d.logger.Error(fmt.Errorf("%v", e))
					}
				}()
				d.ibtpC <- ibtp
				index := ibtp.Index
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						pool.lock.Lock()
						if item := pool.ibtps.Min(); item != nil {
							if item.(*MyTree).index < index+1 {
								pool.ibtps.DeleteMin()
							}

							if item.(*MyTree).index == index+1 {
								d.ibtpC <- item.(*MyTree).ibtp
								pool.ibtps.DeleteMin()
								index++
								pool.time = time.Now()
							}

							// By default, the index will be equalized after 5 seconds
							if time.Now().Sub(pool.time).Seconds() > 5.0 {
								d.ibtpC <- item.(*MyTree).ibtp
								pool.ibtps.DeleteMin()
								index = item.(*MyTree).index
								pool.time = time.Now()
							}
						}
						pool.lock.Unlock()
					case <-d.ctx.Done():
						return
					}
				}
			}(pool, ibtp)
		} else {
			pool.lock.Lock()
			pool.ibtps.ReplaceOrInsert(&MyTree{ibtp: ibtp, index: ibtp.Index})
			pool.lock.Unlock()
		}
		d.gopool.Done()
	}()
}

func (d *DirectAdapter) handleGetInterchainMessage(stream network.Stream, msg *pb.Message) {
	serviceID := string(peermgr.DataToPayload(msg).Data)
	interChain, err := d.appchainadapt.QueryInterchain(serviceID)
	if err != nil {
		d.logger.Error(err)
		return
	}
	data, err := interChain.Marshal()
	if err != nil {
		return
	}

	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	if err := d.peerMgr.AsyncSendWithStream(stream, retMsg); err != nil {
		d.logger.Error(err)
		return
	}
}

func (d *DirectAdapter) handleGetAddressMessage(stream network.Stream, message *pb.Message) {
	addr := d.appchainID

	retMsg := peermgr.Message(pb.Message_ACK, true, []byte(addr))

	err := d.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		d.logger.Error(err)
		return
	}
}
