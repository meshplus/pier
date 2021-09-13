package exchanger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) handleRecover(ibtp *pb.IBTP) (*rpcx.Interchain, error) {
	pierId, err := ex.peerMgr.FindProviders(ibtp.To)
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_ROUTER_INTERCHAIN_GET, true, []byte(ibtp.From))
	res, err := ex.peerMgr.Send(pierId, msg)
	if err != nil {
		return nil, fmt.Errorf("router interchain:%v", err)
	}
	interchain := &pb.Interchain{}
	err = json.Unmarshal(peermgr.DataToPayload(res).Data, interchain)
	if err != nil {
		return nil, err
	}
	ic := &rpcx.Interchain{
		ID:                   interchain.ID,
		InterchainCounter:    interchain.InterchainCounter,
		ReceiptCounter:       interchain.ReceiptCounter,
		SourceReceiptCounter: interchain.SourceReceiptCounter,
	}
	return ic, nil
}

func (ex *Exchanger) handleRouterInterchain(s network.Stream, msg *pb.Message) {
	ic := ex.syncer.QueryInterchainMeta(string(peermgr.DataToPayload(msg).Data))
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

func (ex *Exchanger) handleUnionIBTPFromPier(ibtp *pb.IBTP) error {
	ic, ok := ex.serviceMeta[ibtp.From]
	if !ok {
		ic = ex.syncer.QueryInterchainMeta(ibtp.From)
		ex.serviceMeta[ibtp.From] = ic
	}

	if ibtp.Category() == pb.IBTP_REQUEST {
		if ibtp.Index <= ic.InterchainCounter[ibtp.To] {
			return nil
		}

		ex.askMissingIBTPFromUnionPier(ibtp.From, ibtp.To, ic.InterchainCounter[ibtp.To]+1, ibtp.Index-1, true)
	} else if ibtp.Category() == pb.IBTP_RESPONSE {
		if ibtp.Index <= ic.ReceiptCounter[ibtp.To] {
			return nil
		}

		ex.askMissingIBTPFromUnionPier(ibtp.From, ibtp.To, ic.ReceiptCounter[ibtp.To]+1, ibtp.Index-1, false)
	} else {
		return fmt.Errorf("get unknown ibtp category with id %s and type %s", ibtp.ID(), ibtp.Type)
	}

	if err := ex.sendIBTP(ibtp); err != nil {
		return fmt.Errorf("send ibtp %s to BitXHub: %v", ibtp.ID(), err)
	}

	return nil
}

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleUnionIBTPFromBitXHub(wIbtp *model.WrappedIBTP) {
	ibtp := wIbtp.Ibtp
	entry := ex.logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	})

	var signs []byte
	if err := retry.Retry(func(attempt uint) error {
		var err error
		signs, err = ex.syncer.GetIBTPSigns(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		ex.logger.Panic(err)
	}

	ibtp.Proof = signs

	err := ex.router.Route(ibtp)
	if err != nil {
		entry.WithField("err", err).Warn("Handle union IBTP failed")
		return
	}
	entry.Info("Handle union IBTP successfully")
}

//handleRouterSendIBTPMessage handles IBTP from union interchain network
func (ex *Exchanger) handleRouterSendIBTPMessage(stream network.Stream, msg *pb.Message) {
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

		return ex.handleUnionIBTPFromPier(ibtp)
	}

	if err := handle(); err != nil {
		ex.logger.Error(err)
		return
	}
}

//handleRouterSendIBTPMessage handles get IBTP request from union interchain network
func (ex *Exchanger) handleRouterGetIBTPMessage(stream network.Stream, msg *pb.Message) {
	if err := ex.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), false); err != nil {
		ex.logger.Error(err)
	}
}

//handleRouterGetIBTPReceiptMessage handles get IBTP receipt request from union interchain network
func (ex *Exchanger) handleRouterGetIBTPReceiptMessage(stream network.Stream, msg *pb.Message) {
	if err := ex.queryIBTPFromBitXHubAndSend(stream, string(peermgr.DataToPayload(msg).Data), true); err != nil {
		ex.logger.Error(err)
	}
}

func (ex *Exchanger) queryIBTPFromBitXHubAndSend(stream network.Stream, id string, isReq bool) error {
	ibtp := ex.queryIBTPFromBitXHub(id, isReq)
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
