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
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/sirupsen/logrus"
)

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleIBTP(ibtp *pb.IBTP) {
	err := ex.checker.Check(ibtp)
	if err != nil {
		// todo: send receipt back to bitxhub
		return
	}
	if pb.IBTP_ASSET_EXCHANGE_REDEEM == ibtp.Type || pb.IBTP_ASSET_EXCHANGE_REFUND == ibtp.Type {
		if err := retry.Retry(func(attempt uint) error {
			if err := ex.fetchSignsToIBTP(ibtp); err != nil {
				return err
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	}

	receipt := ex.exec.HandleIBTP(ibtp)
	if receipt == nil {
		return
	}
	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"from":  ibtp.From,
	}).Info("Handle ibtp")

	if err := retry.Retry(func(attempt uint) error {
		_, err = ex.agent.SendIBTP(receipt)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}
}

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleUnionIBTP(ibtp *pb.IBTP) {
	ibtp.From = ex.pierID + "-" + ibtp.From
	var signs []byte
	if err := retry.Retry(func(attempt uint) error {
		var err error
		signs, err = ex.agent.GetIBTPSigns(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}

	ibtp.Proof = signs

	if err := retry.Retry(func(attempt uint) error {
		err := ex.router.Route(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}
	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"from":  ibtp.From,
		"id":    ibtp.ID(),
	}).Info("Route tx successful")
}

func (ex *Exchanger) handleProviderAppchains() error {
	appchains, err := ex.agent.GetAppchains()
	if err != nil {
		return fmt.Errorf("get appchains:%w", err)
	}
	return ex.router.AddAppchains(appchains)
}

//handleRouterSendIBTPMessage handles IBTP from union interchain network
func (ex *Exchanger) handleRouterSendIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	handle := func() error {

		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(msg.Payload.Data); err != nil {
			return fmt.Errorf("unmarshal ibtp: %w", err)
		}

		entry := logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"type":  ibtp.Type,
			"from":  ibtp.From,
			"id":    ibtp.ID(),
		})

		retMsg := peermgr.Message(peerMsg.Message_ACK, true, nil)
		if !ex.router.ExistAppchain(ibtp.To) {
			logger.WithField("appchain", ibtp.To).Errorf("cannot found appchain in relay network")
			retMsg.Payload.Ok = false
		}

		err := ex.peerMgr.AsyncSendWithStream(stream, retMsg)
		if err != nil {
			entry.Error("send back ibtp: %w", err)
			return fmt.Errorf("send back ibtp: %w", err)
		}

		if err := ex.sendIBTP(ibtp); err != nil {
			entry.Infof("Send ibtp: %s", err.Error())
			return err
		}
		return nil
	}

	if err := handle(); err != nil {
		logger.Error(err)
		return
	}
}

func (ex *Exchanger) postHandleIBTP(from string, receipt *pb.IBTP) {
	if receipt == nil {
		retMsg := peermgr.Message(peerMsg.Message_IBTP_RECEIPT_SEND, true, nil)
		err := ex.peerMgr.AsyncSend(from, retMsg)
		if err != nil {
			logger.Error("send back empty ibtp receipt: %w", err)
		}
		return
	}

	//logger.Infof("postHandleIBTP, %s-%s-%d", receipt.From, receipt.To, receipt.Index)
	data, err := receipt.Marshal()
	if err != nil {
		logger.Error("marshal ibtp receipt: %w", err)
	}

	retMsg := peermgr.Message(peerMsg.Message_IBTP_RECEIPT_SEND, true, data)
	err = ex.peerMgr.AsyncSend(from, retMsg)
	if err != nil {
		logger.Error("send back ibtp receipt: %w", err)
	}
}

func (ex *Exchanger) timeCost() func() {
	start := time.Now()
	return func() {
		tc := time.Since(start)
		ex.sendIBTPTimer.Add(tc)
	}
}

func (ex *Exchanger) handleSendIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	ex.ch <- struct{}{}
	go func(msg *peerMsg.Message) {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(msg.Payload.Data); err != nil {
			logger.Error("unmarshal ibtp: %w", err)
			return
		}
		defer ex.timeCost()()
		err := ex.checker.Check(ibtp)
		if err != nil {
			logger.Error("check ibtp: %w", err)
			return
		}

		ex.feedIBTP(ibtp)
		<-ex.ch
	}(msg)

}

func (ex *Exchanger) handleSendIBTPReceiptMessage(stream network.Stream, msg *peerMsg.Message) {
	if msg.Payload.Data == nil {
		return
	}
	receipt := &pb.IBTP{}
	if err := receipt.Unmarshal(msg.Payload.Data); err != nil {
		logger.Error("unmarshal ibtp: %w", err)
		return
	}

	// ignore msg for receipt type
	if receipt.Type == pb.IBTP_RECEIPT_SUCCESS || receipt.Type == pb.IBTP_RECEIPT_FAILURE {
		//logger.Warn("ignore receipt ibtp")
		return
	}

	err := ex.checker.Check(receipt)
	if err != nil {
		logger.Error("check ibtp: %w", err)
		return
	}

	ex.feedReceipt(receipt)

	logger.Info("Receive ibtp receipt from other pier")
}

func (ex *Exchanger) handleGetIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	ibtpID := string(msg.Payload.Data)
	ibtp, err := ex.mnt.QueryIBTP(ibtpID)
	if err != nil {
		logger.Error("Get wrong ibtp id")
		return
	}

	data, err := ibtp.Marshal()
	if err != nil {
		return
	}

	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)

	err = ex.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		logger.Error(err)
	}
}

func (ex *Exchanger) handleNewConnection(dstPierID string) {
	pierID := []byte(ex.pierID)
	msg := peermgr.Message(peerMsg.Message_INTERCHAIN_META_GET, true, pierID)

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	loop := func() error {
		interchainMeta, err := ex.peerMgr.Send(dstPierID, msg)
		if err != nil {
			return err
		}

		if !interchainMeta.Payload.Ok {
			return fmt.Errorf("interchain meta message payload is false")
		}

		if err = json.Unmarshal(interchainMeta.Payload.Data, indices); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Retry(func(attempt uint) error {
		return loop()
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}

	ex.recoverDirect(dstPierID, indices.InterchainIndex, indices.ReceiptIndex)
}

func (ex *Exchanger) handleRecover(ibtp *pb.IBTP) (*rpcx.Interchain, error) {
	pierId, err := ex.peerMgr.FindProviders(ibtp.To)
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(peerMsg.Message_ROUTER_INTERCHAIN_SEND, true, []byte(ibtp.From))
	res, err := ex.peerMgr.Send(pierId, msg)
	if err != nil {
		return nil, fmt.Errorf("router interchain:%v", err)
	}
	interchain := &rpcx.Interchain{}
	err = json.Unmarshal(res.Payload.Data, interchain)
	if err != nil {
		return nil, err
	}
	return interchain, nil
}

func (ex *Exchanger) handleRouterInterchain(s network.Stream, msg *peerMsg.Message) {
	ic := ex.agent.GetInterchainById(string(msg.Payload.Data))
	data, err := ic.Marshal()
	if err != nil {
		panic(err)
	}
	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)
	err = ex.peerMgr.AsyncSendWithStream(s, retMsg)
	if err != nil {
		logger.Error(err)
	}
}

func (ex *Exchanger) handleGetInterchainMessage(stream network.Stream, msg *peerMsg.Message) {
	mntMeta := ex.exec.QueryLatestMeta()
	execMeta := ex.exec.QueryLatestMeta()

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	execLoad, ok := execMeta.Load(string(msg.Payload.Data))
	if ok {
		indices.InterchainIndex = execLoad.(uint64)
	}

	mntLoad, ok := mntMeta.Load(string(msg.Payload.Data))
	if ok {
		indices.InterchainIndex = mntLoad.(uint64)
	}

	data, err := json.Marshal(indices)
	if err != nil {
		panic(err)
	}

	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)
	if err := ex.peerMgr.AsyncSendWithStream(stream, retMsg); err != nil {
		logger.Error(err)
		return
	}
}

func (ex *Exchanger) fetchSignsToIBTP(ibtp *pb.IBTP) error {
	signs, err := ex.agent.GetAssetExchangeSigns(string(ibtp.Extra))
	if err != nil {
		return fmt.Errorf("get asset exchange signs: %w", err)
	}

	ibtp.Extra = signs

	return nil
}
