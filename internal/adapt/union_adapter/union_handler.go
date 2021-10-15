package union_adapter

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

func (ex *UnionAdapter) handleRouterInterchain(s network.Stream, msg *pb.Message) {
	ic, err := ex.bxhAdapter.QueryInterchain(string(peermgr.DataToPayload(msg).Data))
	if err != nil {
		panic(err)
	}
	data, err := ic.Marshal()
	if err != nil {
		panic(err)
	}
	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	err = ex.peerMgr.AsyncSendWithStream(s, retMsg)
	if err != nil {
		ex.logger.Error(err)
	}
}

//handleRouterSendIBTPMessage handles IBTP from union interchain network
func (ex *UnionAdapter) handleRouterSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	handle := func() error {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
			return fmt.Errorf("unmarshal ibtp: %w", err)
		}

		entry := ex.logger.WithFields(logrus.Fields{
			"type": ibtp.Type,
			"id":   ibtp.ID(),
		})

		retMsg := peermgr.Message(pb.Message_ACK, true, nil)

		err := ex.peerMgr.AsyncSendWithStream(stream, retMsg)
		if err != nil {
			return fmt.Errorf("send back ibtp: %w", err)
		}
		entry.Infof("Receive ibtp successfully")
		ex.ibtpC <- ibtp
		return nil
	}

	if err := handle(); err != nil {
		ex.logger.Error(err)
		return
	}
}

//handleRouterSendIBTPMessage handles get IBTP request from union interchain network
func (ex *UnionAdapter) handleRouterGetIBTPMessage(stream network.Stream, msg *pb.Message) {
	if err := ex.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), false); err != nil {
		ex.logger.Error(err)
	}
}

//handleRouterGetIBTPReceiptMessage handles get IBTP receipt request from union interchain network
func (ex *UnionAdapter) handleRouterGetIBTPReceiptMessage(stream network.Stream, msg *pb.Message) {
	if err := ex.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), true); err != nil {
		ex.logger.Error(err)
	}
}

func (ex *UnionAdapter) queryIBTPFromBitXHubAndSend(stream network.Stream, id string, isReq bool) error {
	ibtp, err := ex.bxhAdapter.QueryIBTP(id, isReq)
	if err != nil {
		return fmt.Errorf("bxhAdapter queryIBTP %s: %w", ibtp.ID(), err)
	}
	data, err := ibtp.Marshal()
	if err != nil {
		return fmt.Errorf("marshal ibtp %s: %w", ibtp.ID(), err)
	}

	ex.logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	}).Info("Sending IBTP")

	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	err = ex.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		return fmt.Errorf("send ibtp %s: %w", ibtp.ID(), err)
	}

	return nil
}
