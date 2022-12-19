package union_adapter

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

func (u *UnionAdapter) handleRouterInterchain(s network.Stream, msg *pb.Message) {
	ic, err := u.bxhAdapter.QueryInterchain(string(peermgr.DataToPayload(msg).Data))
	if err != nil {
		panic(err)
	}
	data, err := ic.Marshal()
	if err != nil {
		panic(err)
	}
	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	err = u.peerMgr.AsyncSendWithStream(s, retMsg)
	if err != nil {
		u.logger.Error(err)
	}
}

func (u *UnionAdapter) handleGetAddressMessage(stream network.Stream, message *pb.Message) {
	addr := u.bxhId

	retMsg := peermgr.Message(pb.Message_ACK, true, []byte(addr))

	err := u.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		u.logger.Error(err)
		return
	}
}

//handleRouterSendIBTPMessage handles IBTP from union interchain network
func (u *UnionAdapter) handleRouterSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	handle := func() error {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			return fmt.Errorf("unmarshal ibtp: %w", err)
		}

		entry := u.logger.WithFields(logrus.Fields{
			"type": ibtp.Type,
			"id":   ibtp.ID(),
		})

		retMsg := peermgr.Message(pb.Message_ACK, true, nil)

		err := u.peerMgr.AsyncSendWithStream(stream, retMsg)
		if err != nil {
			return fmt.Errorf("send back ibtp: %w", err)
		}
		entry.Infof("Receive ibtp successfully")
		u.ibtpC <- ibtp
		return nil
	}

	if err := handle(); err != nil {
		u.logger.Error(err)
		return
	}
}

//handleRouterSendIBTPMessage handles get IBTP request from union interchain network
func (u *UnionAdapter) handleRouterGetIBTPMessage(stream network.Stream, msg *pb.Message) {
	if err := u.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), true); err != nil {
		u.logger.Error(err)
	}
}

//handleRouterGetIBTPReceiptMessage handles get IBTP receipt request from union interchain network
func (u *UnionAdapter) handleRouterGetIBTPReceiptMessage(stream network.Stream, msg *pb.Message) {
	if err := u.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), false); err != nil {
		u.logger.Error(err)
	}
}

func (u *UnionAdapter) queryIBTPFromBitXHubAndSend(stream network.Stream, id string, isReq bool) error {
	ibtp, err := u.bxhAdapter.QueryIBTP(id, isReq)
	if err != nil {
		return fmt.Errorf("bxhAdapter queryIBTP %s: %w", id, err)
	}
	data, err := ibtp.Marshal()
	if err != nil {
		return fmt.Errorf("marshal ibtp %s: %w", ibtp.ID(), err)
	}

	u.logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	}).Info("Sending IBTP")

	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	err = u.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		return fmt.Errorf("send ibtp %s: %w", ibtp.ID(), err)
	}

	return nil
}
