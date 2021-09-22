package exchanger

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleIBTP(wIbtp *model.WrappedIBTP, entry logrus.FieldLogger) {
	ibtp := wIbtp.Ibtp
	err := ex.checker.Check(ibtp)
	if err != nil {
		// todo: send receipt back to bitxhub
		return
	}
	entry.Debugf("IBTP pass check")

	receipt, err := ex.exec.ExecuteIBTP(wIbtp)
	if err != nil {
		ex.logger.Errorf("execute ibtp error:%s", err.Error())
	}
	if receipt == nil {
		ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()}).Info("Handle ibtp receipt success")
		return
	}

sendReceiptLoop:
	for {
		err = ex.syncer.SendIBTP(receipt)
		if err != nil {
			ex.logger.Errorf("send ibtp error: %s", err.Error())
			if errors.Is(err, syncer.ErrMetaOutOfDate) {
				ex.updateSourceReceiptMeta(ibtp.To)
				return
			}
			if strings.Contains(err.Error(), "rollback") {
				ex.exec.Rollback(ibtp, false)
				return
			}
			// if sending receipt failed, try to get new receipt from appchain and retry
		queryLoop:
			for {
				receipt, err = ex.exec.QueryIBTPReceipt(ibtp)
				if err != nil {
					ex.logger.Errorf("Query ibtp receipt for %s error: %s", ibtp.ID(), err.Error())
					time.Sleep(1 * time.Second)
					continue queryLoop
				}
				time.Sleep(1 * time.Second)
				continue sendReceiptLoop
			}
		}
		break
	}
	ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()}).Info("Handle ibtp success")
}

func (ex *Exchanger) applyReceipt(wIbtp *model.WrappedIBTP, entry logrus.FieldLogger) {
	ibtp := wIbtp.Ibtp
	index := ex.serviceMeta[ibtp.From].ReceiptCounter[ibtp.To]
	if index >= ibtp.Index {
		entry.Infof("Ignore ibtp callback, expected index %d", index+1)
		return
	}

	if index+1 < ibtp.Index {
		entry.Infof("Get missing ibtp receipt, expected index %d", index+1)
		// todo: need to handle missing ibtp receipt or not?
		return
	}
	ex.handleIBTP(wIbtp, entry)
	ex.serviceMeta[ibtp.From].ReceiptCounter[ibtp.To] = ibtp.Index
}

func (ex *Exchanger) applyInterchain(wIbtp *model.WrappedIBTP, entry logrus.FieldLogger) {
	ibtp := wIbtp.Ibtp
	_, ok := ex.serviceMeta[ibtp.To]
	if !ok {
		ex.serviceMeta[ibtp.To] = &pb.Interchain{
			ID:                      ibtp.To,
			InterchainCounter:       make(map[string]uint64),
			ReceiptCounter:          make(map[string]uint64),
			SourceInterchainCounter: make(map[string]uint64),
			SourceReceiptCounter:    make(map[string]uint64),
		}
	}
	index := ex.serviceMeta[ibtp.To].SourceInterchainCounter[ibtp.From]
	if index >= ibtp.Index {
		entry.Infof("Ignore ibtp, expected %d", index+1)
		return
	}

	if index+1 < ibtp.Index {
		entry.Info("Get missing ibtp")
		servicePair := fmt.Sprintf("%s-%s", ibtp.From, ibtp.To)
		if err := ex.handleMissingIBTPFromSyncer(servicePair, index+1, ibtp.Index); err != nil {
			entry.WithField("err", err).Error("Handle missing ibtp")
			return
		}
	}
	ex.handleIBTP(wIbtp, entry)
	ex.serviceMeta[ibtp.To].SourceInterchainCounter[ibtp.From] = ibtp.Index
}

func (ex *Exchanger) handleRollback(ibtp *pb.IBTP, ibtpId string) {
	var err error
	if ibtp == nil {
		if err := retry.Retry(func(attempt uint) error {
			ibtp, err = ex.mnt.QueryIBTP(ibtpId)
			if err != nil {
				ex.logger.Warnf("query ibtp %s: %v", err)
				return err
			}
			return nil
		}, strategy.Wait(time.Second*1)); err != nil {
			ex.logger.Panic(err)
		}
	}
	if ibtp.Category() == pb.IBTP_RESPONSE {
		// if this is receipt type of ibtp, no need to rollback
		return
	}
	ex.feedIBTPReceipt(&model.WrappedIBTP{Ibtp: ibtp, IsValid: false})
}

func (ex *Exchanger) postHandleIBTP(from string, receipt *pb.IBTP) {
	if receipt == nil {
		retMsg := peermgr.Message(pb.Message_IBTP_RECEIPT_SEND, true, nil)
		err := ex.peerMgr.AsyncSend(from, retMsg)
		if err != nil {
			ex.logger.Errorf("Send back empty ibtp receipt: %s", err.Error())
		}
		return
	}

	data, _ := receipt.Marshal()
	retMsg := peermgr.Message(pb.Message_IBTP_RECEIPT_SEND, true, data)
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

func (ex *Exchanger) handleSendIBTPMessage(stream network.Stream, msg *pb.Message) {
	ex.ch <- struct{}{}
	go func(msg *pb.Message) {
		wIbtp := &model.WrappedIBTP{}
		if err := json.Unmarshal(peermgr.DataToPayload(msg).Data, wIbtp); err != nil {
			ex.logger.Errorf("Unmarshal ibtp: %s", err.Error())
			return
		}
		defer ex.timeCost()()
		err := ex.checker.Check(wIbtp.Ibtp)
		if err != nil {
			ex.logger.Error("check ibtp: %w", err)
			return
		}

		ex.feedIBTP(wIbtp)
		<-ex.ch
	}(msg)
}

func (ex *Exchanger) handleSendIBTPReceiptMessage(stream network.Stream, msg *pb.Message) {
	if peermgr.DataToPayload(msg).Data == nil {
		return
	}
	receipt := &pb.IBTP{}
	if err := receipt.Unmarshal(peermgr.DataToPayload(msg).Data); err != nil {
		ex.logger.Error("unmarshal ibtp: %w", err)
		return
	}

	// ignore msg for receipt type
	if receipt.Type == pb.IBTP_RECEIPT_SUCCESS || receipt.Type == pb.IBTP_RECEIPT_FAILURE {
		//ex.logger.Warn("ignore receipt ibtp")
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

func (ex *Exchanger) handleGetIBTPMessage(stream network.Stream, msg *pb.Message) {
	ibtpID := string(peermgr.DataToPayload(msg).Data)
	ibtp, err := ex.mnt.QueryIBTP(ibtpID)
	if err != nil {
		ex.logger.Error("Get wrong ibtp id")
		return
	}

	data, err := ibtp.Marshal()
	if err != nil {
		return
	}

	retMsg := peermgr.Message(pb.Message_ACK, true, data)

	err = ex.peerMgr.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		ex.logger.Error(err)
	}
}

func (ex *Exchanger) handleNewConnection(dstPierID string) {
	appchainMethod := []byte(ex.appchainDID)
	msg := peermgr.Message(pb.Message_INTERCHAIN_META_GET, true, appchainMethod)

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	loop := func() error {
		interchainMeta, err := ex.peerMgr.Send(dstPierID, msg)
		if err != nil {
			return err
		}

		if !peermgr.DataToPayload(interchainMeta).Ok {
			return fmt.Errorf("interchain meta message payload is false")
		}

		if err = json.Unmarshal(peermgr.DataToPayload(interchainMeta).Data, indices); err != nil {
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

func (ex *Exchanger) handleGetInterchainMessage(stream network.Stream, msg *pb.Message) {
	mntMeta := ex.mnt.QueryOuterMeta()
	execMeta := ex.exec.QueryInterchainMeta()

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	execLoad, ok := execMeta[string(peermgr.DataToPayload(msg).Data)]
	if ok {
		indices.InterchainIndex = execLoad
	}

	mntLoad, ok := mntMeta[string(peermgr.DataToPayload(msg).Data)]
	if ok {
		indices.InterchainIndex = mntLoad
	}

	data, err := json.Marshal(indices)
	if err != nil {
		panic(err)
	}

	retMsg := peermgr.Message(pb.Message_ACK, true, data)
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

func (ex *Exchanger) getBitXHubChainID() uint64 {
	var (
		chainID uint64
		err     error
	)

	if err := retry.Retry(func(attempt uint) error {
		chainID, err = ex.syncer.GetChainID()
		if err != nil {
			ex.logger.Warnf("get bitxhub chain id: %w", err)
		}

		return err
	}, strategy.Wait(time.Second*2)); err != nil {
		ex.logger.Panicf("get bitxhub chain id: %w", err)
	}

	return chainID
}
