package direct_adapter

import (
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/utils"
)

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

func (d *DirectAdapter) handleSendIBTPMessage(_ network.Stream, msg *pb.Message) {
	d.gopool.Add()
	go func() {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			d.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		servicePair := ibtp.From + ibtp.To + ibtp.Category().String()
		act, loaded := d.ibtps.LoadOrStore(servicePair, utils.NewPool(utils.DirectDegree))
		pool := act.(*utils.Pool)

		if !loaded {
			go func(pool *utils.Pool, ibtp *pb.IBTP) {
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
						pool.Lock.Lock()
						if item := pool.Ibtps.Min(); item != nil {
							if item.(*utils.MyTree).Index < index+1 {
								pool.Ibtps.DeleteMin()
							}

							if item.(*utils.MyTree).Index == index+1 {
								d.ibtpC <- item.(*utils.MyTree).Ibtp
								pool.Ibtps.DeleteMin()
								index++
								pool.Time = time.Now()
							}

							// By default, the index will be equalized after 5 seconds
							if time.Since(pool.Time).Seconds() > 5.0 {
								d.ibtpC <- item.(*utils.MyTree).Ibtp
								pool.Ibtps.DeleteMin()
								index = item.(*utils.MyTree).Index
								pool.Time = time.Now()
							}
						}
						pool.Lock.Unlock()
					case <-d.ctx.Done():
						return
					}
				}
			}(pool, ibtp)
		} else {
			pool.Lock.Lock()
			pool.Ibtps.ReplaceOrInsert(&utils.MyTree{Ibtp: ibtp, Index: ibtp.Index})
			pool.Lock.Unlock()
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

func (d *DirectAdapter) handleGetAddressMessage(stream network.Stream, _ *pb.Message) {
	addr := d.appchainID

	retMsg := peermgr.Message(pb.Message_ACK, true, []byte(addr))

	err := d.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		d.logger.Error(err)
		return
	}
}
