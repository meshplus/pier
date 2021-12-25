package direct_adapter

import (
	"fmt"
	"sync"

	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
)

type Pool struct {
	req_ibtps  *sync.Map
	resp_ibtps *sync.Map
	ch         chan *pb.IBTP
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
		req_ibtps:  &sync.Map{},
		resp_ibtps: &sync.Map{},
		ch:         make(chan *pb.IBTP, 40960),
	}
}

func (d *DirectAdapter) handleSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	//defer d.timeCost()()
	d.gopool.Add()
	go func() {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			d.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		servicePair := ibtp.From + ibtp.To
		act, loaded := d.ibtps.LoadOrStore(servicePair, NewPool())
		pool := act.(*Pool)
		pool.ch <- ibtp

		if !loaded {
			go func(pool *Pool, ibtp *pb.IBTP, servicePair string) {
				defer func() {
					if e := recover(); e != nil {
						d.logger.Error(fmt.Errorf("%v", e))
					}
				}()
				for ibtp := range pool.ch {
					if ibtp.Category() == pb.IBTP_REQUEST {
						load, _ := d.maxIndexMap.LoadOrStore(servicePair, uint64(0))
						if ibtp.Index == load.(uint64)+1 {
							d.ibtpC <- ibtp
							d.maxIndexMap.Store(servicePair, ibtp.Index)
							pool.req_ibtps.LoadAndDelete(ibtp.Index)
						}

						if ibtp.Index < load.(uint64)+1 {
							continue
						}

						if ibtp.Index > load.(uint64)+1 {
							pool.req_ibtps.Store(ibtp.Index, ibtp)
							for i := load.(uint64) + 1; i <= ibtp.Index; i++ {
								req_ibtp, ok := pool.req_ibtps.LoadAndDelete(i)
								if !ok {
									break
								}
								d.ibtpC <- req_ibtp.(*pb.IBTP)
								d.maxIndexMap.Store(servicePair, i)
							}
						}

					} else {
						load, _ := d.maxIndexReceiptMap.LoadOrStore(servicePair, uint64(0))
						if ibtp.Index == load.(uint64)+1 {
							d.ibtpC <- ibtp
							d.maxIndexReceiptMap.Store(servicePair, ibtp.Index)
							pool.resp_ibtps.LoadAndDelete(ibtp.Index)
						}

						if ibtp.Index < load.(uint64)+1 {
							continue
						}

						if ibtp.Index > load.(uint64)+1 {
							pool.resp_ibtps.Store(ibtp.Index, ibtp)
							for i := load.(uint64) + 1; i <= ibtp.Index; i++ {
								resp_ibtp, ok := pool.resp_ibtps.LoadAndDelete(i)
								if !ok {
									break
								}
								d.ibtpC <- resp_ibtp.(*pb.IBTP)
								d.maxIndexReceiptMap.Store(servicePair, i)
							}
						}
					}
				}
			}(pool, ibtp, servicePair)
		}
		d.gopool.Done()
	}()
	//d.sendIBTPCounter.Inc()
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
		panic(err)
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
