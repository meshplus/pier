package exchanger

import (
	"encoding/json"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/meshplus/bitxhub-model/pb"
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/sirupsen/logrus"
)

const (
	ErrorCode   = "ERROR"
	ConfirmCode = "OK"
)

// handleIBTP handle ibtps from bitxhub
func (ex *Exchanger) handleIBTP(ibtp *pb.IBTP) {
	ok, err := ex.checker.Check(ibtp)
	if err != nil || !ok {
		// todo: send receipt back to bitxhub
		return
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

// handleIBTPMessage handle ibtp message from another pier
func (ex *Exchanger) handleIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	ibtp := &pb.IBTP{}
	if err := ibtp.Unmarshal(msg.Data); err != nil {
		logger.Warnf("Received invalid ibtp message: %s", err.Error())
		ex.sentACKMsg(stream, ErrorCode)
		return
	}

	ok, err := ex.checker.Check(ibtp)
	if err != nil || !ok {
		ex.sentACKMsg(stream, ErrorCode)
		return
	}

	// send ibtp_ack to counterpart chain
	ex.sentACKMsg(stream, ConfirmCode)

	receipt := ex.exec.HandleIBTP(ibtp)
	data, err := receipt.Marshal()
	if err != nil {
		logger.Errorf("invalid receipt message: %s", err.Error())
		return
	}

	retMsg := &peerMsg.Message{
		Type: peerMsg.Message_IBTP_RECEIPT,
		Data: data,
	}

	if err := retry.Retry(func(attempt uint) error {
		return ex.peerMgr.SendWithStream(stream, retMsg)
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}
}

func (ex *Exchanger) handleQueryIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	ibtpID := string(msg.Data)
	ibtp, err := ex.mnt.QueryIBTP(ibtpID)
	if err != nil {
		logger.Error("get wrong ibtp id")
		return
	}
	data, err := ibtp.Marshal()
	if err != nil {
		return
	}

	retMsg := &peerMsg.Message{
		Type: peerMsg.Message_QUERY_IBTP_ACK,
		Data: data,
	}

	if err := retry.Retry(func(attempt uint) error {
		return ex.peerMgr.SendWithStream(stream, retMsg)
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}
}

func (ex *Exchanger) handleNewConnection(dstPierID string) {
	msg := &peerMsg.Message{
		Type: peerMsg.Message_INTERCHAIN,
		Data: []byte(ex.pierID),
	}

	indices := &struct {
		InterchainIndex uint64
		ReceiptIndex    uint64
	}{}
	loop := func() error {
		receipt, err := ex.peerMgr.Send(dstPierID, msg)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(receipt.Data, indices); err != nil {
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

func (ex *Exchanger) handleInterchainMessage(stream network.Stream, msg *peerMsg.Message) {
	mntMeta := ex.exec.QueryLatestMeta()
	execMeta := ex.exec.QueryLatestMeta()

	indices := &struct {
		InterchainIndex uint64
		ReceiptIndex    uint64
	}{}
	indices.InterchainIndex = execMeta[string(msg.GetData())]
	indices.ReceiptIndex = mntMeta[string(msg.GetData())]

	data, err := json.Marshal(indices)
	if err != nil {
		return
	}
	retMsg := &peerMsg.Message{
		Type: peerMsg.Message_INTERCHAIN_ACK,
		Data: data,
	}
	if err := ex.peerMgr.SendWithStream(stream, retMsg); err != nil {
		return
	}
}

func (ex *Exchanger) sentACKMsg(stream network.Stream, statusCode string) {
	msg := &peerMsg.Message{
		Type: peerMsg.Message_IBTP_ACK,
		Data: []byte(statusCode),
	}

	if err := retry.Retry(func(attempt uint) error {
		return ex.peerMgr.SendWithStream(stream, msg)
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Panic(err)
	}
}
