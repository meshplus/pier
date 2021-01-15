package exchanger

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) recoverRelay() {
	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryOuterMeta()
	for to, idx := range mntMeta {
		beginIndex, ok := ex.interchainCounter[to]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingIBTPFromMnt(to, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{
				"address": to,
				"error":   err.Error(),
			}).Panic("Get missing receipt from contract")
		}
	}

	// recover unsent receipt to counterpart chain
	execMeta := ex.exec.QueryMeta()
	for from, idx := range execMeta {
		beginIndex, ok := ex.sourceReceiptCounter[from]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingReceipt(from, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{
				"address": from,
				"error":   err.Error(),
			}).Panic("Get missing receipt from contract")
		}
	}
}

func (ex *Exchanger) recoverDirect(dstPierID string, interchainIndex uint64, receiptIndex uint64) {
	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryOuterMeta()
	index, ok := mntMeta[dstPierID]
	if !ok {
		ex.logger.Infof("Appchain %s not exist", dstPierID)
		return
	}
	if err := ex.handleMissingIBTPFromMnt(dstPierID, interchainIndex+1, index+1); err != nil {
		ex.logger.WithFields(logrus.Fields{
			"address": dstPierID,
			"error":   err.Error(),
		}).Error("Handle missing ibtp")
	}

	// recoverDirect unsent receipt to counterpart chain
	execMeta := ex.exec.QueryMeta()
	idx := execMeta[dstPierID]
	if err := ex.handleMissingReceipt(dstPierID, receiptIndex+1, idx+1); err != nil {
		ex.logger.WithFields(logrus.Fields{
			"address": dstPierID,
			"error":   err.Error(),
		}).Panic("Get missing receipt from contract")
	}
}

func (ex *Exchanger) handleMissingIBTPFromMnt(to string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"to":    to,
			"index": begin,
		}).Info("Get missing event from contract")

		ibtp, err := ex.mnt.QueryIBTP(fmt.Sprintf("%s-%s-%d", ex.pierID, to, begin))
		if err != nil {
			return fmt.Errorf("fetch ibtp:%w", err)
		}

		if err := ex.sendIBTP(ibtp); err != nil {
			return err
		}
		ex.interchainCounter[ibtp.To] = ibtp.Index
	}

	return nil
}

func (ex *Exchanger) handleMissingIBTPFromSyncer(from string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"from":  from,
			"index": begin,
		}).Info("Get missing event from bitxhub")

		ibtp, err := ex.syncer.QueryIBTP(fmt.Sprintf("%s-%s-%d", from, ex.pierID, begin))
		if err != nil {
			return fmt.Errorf("fetch ibtp:%w", err)
		}

		ex.handleIBTP(ibtp)
		ex.executorCounter[ibtp.From] = ibtp.Index
	}

	return nil
}

func (ex *Exchanger) handleMissingReceipt(from string, begin uint64, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing receipt is required >= 1")
	}

	entry := ex.logger.WithFields(logrus.Fields{
		"from": from,
	})

	for ; begin < end; begin++ {
		entry = entry.WithFields(logrus.Fields{
			"index": begin,
		})
		original, err := ex.queryIBTP(from, begin)
		if err != nil {
			return err
		}

		receipt, err := ex.exec.QueryIBTPReceipt(from, begin, original)
		if err != nil {
			entry.WithField("error", err.Error()).Error("Get missing execution receipt result")
			return err
		}

		// send receipt back to counterpart chain
		if err := ex.sendIBTP(receipt); err != nil {
			entry.WithField("error", err).Error("Send execution receipt to counterpart chain")
			return err
		}
		ex.sourceReceiptCounter[from] = receipt.Index
	}
	return nil
}
