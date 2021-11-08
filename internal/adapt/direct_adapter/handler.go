package direct_adapter

import (
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
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

func (d *DirectAdapter) handleSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	go func(msg *pb.Message) {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			d.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		defer d.timeCost()()
		defer d.lock.Unlock()

		d.lock.Lock()
		// make sure the ibtpC's index is incremented
		_, _, srviceID := ibtp.ParseTo()
		if ibtp.Index > d.maxIndexMap[srviceID] {
			d.ibtpC <- ibtp
			d.maxIndexMap[srviceID] = ibtp.Index
		} else {
			// if receive smaller index, put it in the ibtpCache
			d.ibtpCache.Add(ibtp.Index, ibtp)
		}
	}(msg)
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

func (d *DirectAdapter) timeCost() func() {
	start := time.Now()
	return func() {
		tc := time.Since(start)
		d.sendIBTPTimer.Add(tc)
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
