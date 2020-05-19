package exchanger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/meshplus/bitxhub-model/pb"
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
func (ex *Exchanger) handleSendIBTPMessage(stream network.Stream, msg *peerMsg.Message) {
	handle := func() error {
		ibtp := &pb.IBTP{}
		if err := ibtp.Unmarshal(msg.Payload.Data); err != nil {
			return fmt.Errorf("unmarshal ibtp: %w", err)
		}

		err := ex.checker.Check(ibtp)
		if err != nil {
			return fmt.Errorf("check ibtp: %w", err)
		}

		receipt := ex.exec.HandleIBTP(ibtp)
		data, err := receipt.Marshal()
		if err != nil {
			return fmt.Errorf("marshal ibtp receipt: %w", err)
		}

		retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)

		err = ex.peerMgr.SendWithStream(stream, retMsg)
		if err != nil {
			return fmt.Errorf("send back ibtp: %w", err)
		}

		return nil
	}

	if err := handle(); err != nil {
		logger.Error(err)
		return
	}

	logger.Info("Handle ibtp from other pier")
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

	err = ex.peerMgr.SendWithStream(stream, retMsg)
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

func (ex *Exchanger) handleGetInterchainMessage(stream network.Stream, msg *peerMsg.Message) {
	mntMeta := ex.exec.QueryLatestMeta()
	execMeta := ex.exec.QueryLatestMeta()

	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}

	indices.InterchainIndex = execMeta[string(msg.Payload.Data)]
	indices.ReceiptIndex = mntMeta[string(msg.Payload.Data)]

	data, err := json.Marshal(indices)
	if err != nil {
		panic(err)
	}

	retMsg := peermgr.Message(peerMsg.Message_ACK, true, data)
	if err := ex.peerMgr.SendWithStream(stream, retMsg); err != nil {
		logger.Error(err)
		return
	}
}
