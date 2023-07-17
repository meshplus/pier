package exchanger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/meshplus/pier/internal/repo"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleIBTP(wIbtp *model.WrappedIBTP) {
	ibtp := wIbtp.Ibtp
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
			ex.logger.Panic(err)
		}
	}

	receipt, err := ex.exec.ExecuteIBTP(wIbtp)
	if err != nil {
		ex.logger.Errorf("execute ibtp error:%s", err.Error())
		// todo: 如果是index error，跳过后面的处理，直接更新seqNo；
	}
	if receipt == nil {
		ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()}).Info("Handle ibtp receipt success")
		return
	}
	ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()}).Info("Handle ibtp success")

	err = ex.syncer.SendIBTP(receipt)
	if err != nil {
		ex.logger.Errorf("send ibtp error:%v", err)
	}
}

func (ex *Exchanger) applyReceipt(wIbtp *model.WrappedIBTP) {
	ex.feedIBTPReceipt(wIbtp)
}

func (ex *Exchanger) applyInterchain(wIbtp *model.WrappedIBTP, entry logrus.FieldLogger) {
	ibtp := wIbtp.Ibtp
	from := ibtp.From
	index := ex.executorCounter[from]
	if index >= ibtp.Index {
		entry.Infof("Ignore ibtp, expected %d", index+1)
		return
	}

	// 应用链的inCounter比中继链上小不止1，那么裂开了
	if index+1 < ibtp.Index {
		entry.Info("Get missing ibtp")

		if err := ex.handleMissingIBTPFromSyncer(from, index+1, ibtp.Index, entry); err != nil {
			entry.WithField("err", err).Error("Handle missing ibtp")
			return
		}
	}
	if ex.mode == repo.UnionMode {
		ex.handleUnionIBTP(wIbtp)
	} else {
		ex.handleIBTP(wIbtp)
	}
	entry.Infof("update exchanger.executorCounter[%s]=%d", from, ibtp.Index)
	ex.executorCounter[from] = ibtp.Index

}

func (ex *Exchanger) handleRollback(ibtp *pb.IBTP) {
	if ibtp.Category() == pb.IBTP_RESPONSE {
		// if this is receipt type of ibtp, no need to rollback
		return
	}
	ex.feedIBTPReceipt(&model.WrappedIBTP{Ibtp: ibtp, IsValid: false})
	ex.logger.Infof("Rollback in source chain successfully")
}

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleUnionIBTP(wIbtp *model.WrappedIBTP) {
	if !wIbtp.IsValid {
		return
	}
	ibtp := wIbtp.Ibtp
	ibtp.From = ex.pierID + "-" + ibtp.From
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

	if err := retry.Retry(func(attempt uint) error {
		err := ex.router.Route(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		ex.logger.Panic(err)
	}
	ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "from": ibtp.From, "id": ibtp.ID()}).Info("Route tx successful")
}

func (ex *Exchanger) handleProviderAppchains() error {
	appchains, err := ex.syncer.GetAppchains()
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
		entry := ex.logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"type":  ibtp.Type,
			"from":  ibtp.From,
			"to":    ibtp.To,
			"id":    ibtp.ID(),
		})

		retMsg := peermgr.Message(peerMsg.Message_ACK, true, nil)
		if !ex.router.ExistAppchain(ibtp.To) {
			entry.WithField("appchain", ibtp.To).Errorf("cannot found appchain in relay network")
			retMsg.Payload.Ok = false
		}

		err := ex.peerMgr.AsyncSendWithStream(stream, retMsg)
		if err != nil {
			return fmt.Errorf("send back ibtp: %w", err)
		}
		entry.Infof("receive ibtp successfully")

		if err := ex.sendIBTP(ibtp); err != nil {
			return fmt.Errorf("send ibtp: %v", err)
		}
		return nil
	}

	if err := handle(); err != nil {
		ex.logger.Error(err)
		return
	}
}

func (ex *Exchanger) postHandleIBTP(from string, receipt *pb.IBTP) {
	if receipt == nil {
		retMsg := peermgr.Message(peerMsg.Message_IBTP_RECEIPT_SEND, true, nil)
		err := ex.peerMgr.AsyncSend(from, retMsg)
		if err != nil {
			ex.logger.Errorf("Send back empty ibtp receipt: %s", err.Error())
		}
		return
	}

	data, _ := receipt.Marshal()
	retMsg := peermgr.Message(peerMsg.Message_IBTP_RECEIPT_SEND, true, data)
	if err := ex.peerMgr.AsyncSend(from, retMsg); err != nil {
		ex.logger.Errorf("Send back ibtp receipt: %s", err.Error())
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
		defer func() {
			if err := recover(); err != nil {
				ex.logger.Errorf("send ibtp:%v", err)
			}
		}()
		ibtp := &pb.IBTP{}
		wIbtp := &model.WrappedIBTP{IsValid: true}
		if err := ibtp.Unmarshal(msg.Payload.Data); err != nil {
			ex.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		defer ex.timeCost()()
		err := ex.checker.Check(ibtp)
		if err != nil {
			ex.logger.Error("check ibtp: %w", err)
			wIbtp.IsValid = false
		}
		wIbtp.Ibtp = ibtp
		ex.feedIBTP(wIbtp)
		<-ex.ch
	}(msg)
}

func (ex *Exchanger) handleSendIBTPReceiptMessage(stream network.Stream, msg *peerMsg.Message) {
	if msg.Payload.Data == nil {
		return
	}
	receipt := &pb.IBTP{}
	if err := receipt.Unmarshal(msg.Payload.Data); err != nil {
		ex.logger.Error("unmarshal ibtp: %w", err)
		return
	}

	// ignore msg for receipt type
	if receipt.Type == pb.IBTP_RECEIPT_SUCCESS || receipt.Type == pb.IBTP_RECEIPT_FAILURE {
		ex.logger.Warn("ignore receipt ibtp")
		return
	}

	err := ex.checker.Check(receipt)
	if err != nil {
		ex.logger.Error("check ibtp: %w", err)
		return
	}

	ex.feedReceipt(receipt)

	ex.logger.Info("Receive ibtp receipt from other pier")
}

func (ex *Exchanger) handleGetIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	ibtpID := string(msg.Payload.Data)
	ibtp, err := ex.mnt.QueryIBTP(ibtpID)
	if err != nil {
		ex.logger.Error("Get wrong ibtp id")
		return
	}

	data, err := ibtp.Marshal()
	if err != nil {
		return
	}

	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)

	err = ex.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		ex.logger.Error(err)
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
		ex.logger.Panic(err)
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
	interchain := &pb.Interchain{}
	err = json.Unmarshal(res.Payload.Data, interchain)
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

func (ex *Exchanger) handleRouterInterchain(s network.Stream, msg *peerMsg.Message) {
	ic := ex.syncer.GetInterchainById(string(msg.Payload.Data))
	data, err := ic.Marshal()
	if err != nil {
		panic(err)
	}
	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)
	err = ex.peerMgr.AsyncSendWithStream(s, retMsg)
	if err != nil {
		ex.logger.Error(err)
	}
}

func (ex *Exchanger) handleGetInterchainMessage(stream network.Stream, msg *peerMsg.Message) {
	mntMeta := ex.mnt.QueryOuterMeta()
	execMeta := ex.exec.QueryInterchainMeta()

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	execLoad, ok := execMeta[string(msg.Payload.Data)]
	if ok {
		indices.InterchainIndex = execLoad
	}

	mntLoad, ok := mntMeta[string(msg.Payload.Data)]
	if ok {
		indices.InterchainIndex = mntLoad
	}

	data, err := json.Marshal(indices)
	if err != nil {
		panic(err)
	}

	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)
	if err := ex.peerMgr.AsyncSendWithStream(stream, retMsg); err != nil {
		ex.logger.Error(err)
		return
	}
}

func (ex *Exchanger) fetchSignsToIBTP(ibtp *pb.IBTP) error {
	signs, err := ex.syncer.GetAssetExchangeSigns(string(ibtp.Extra))
	if err != nil {
		return fmt.Errorf("get asset exchange signs: %w", err)
	}

	ibtp.Extra = signs

	return nil
}
