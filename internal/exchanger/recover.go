package exchanger

import (
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
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
			ex.logger.WithFields(logrus.Fields{"address": to, "error": err.Error()}).Panic("Get missing receipt from contract")
		}
	}

	// recover unsent receipt to counterpart chain
	execMeta := ex.exec.QueryInterchainMeta()
	for from, idx := range execMeta {
		beginIndex, ok := ex.sourceReceiptCounter[from]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingReceipt(from, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{"address": from, "error": err.Error()}).Panic("Get missing receipt from contract")
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
		ex.logger.WithFields(logrus.Fields{"address": dstPierID, "error": err.Error()}).Error("Handle missing ibtp")
	}

	// recoverDirect unsent receipt to counterpart chain
	execMeta := ex.exec.QueryInterchainMeta()
	idx := execMeta[dstPierID]
	if err := ex.handleMissingReceipt(dstPierID, receiptIndex+1, idx+1); err != nil {
		ex.logger.WithFields(logrus.Fields{"address": dstPierID, "error": err.Error()}).Panic("Get missing receipt from contract")
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

		if err := retry.Retry(func(attempt uint) error {
			ibtp, err := ex.mnt.QueryIBTP(fmt.Sprintf("%s-%s-%d", ex.appchainDID, to, begin))
			if err != nil {
				ex.logger.Errorf("Fetch ibtp: %s", err.Error())
				return err
			}

			if err := ex.sendIBTP(ibtp); err != nil {
				ex.logger.Errorf("Send ibtp: %s", err.Error())
				// if err occurs, try to resend this ibtp
				return err
			}
			ex.interchainCounter[ibtp.To] = ibtp.Index
			return nil
		}, strategy.Wait(500*time.Millisecond), strategy.Limit(10)); err != nil {
			return err
		}
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

		ibtpID := fmt.Sprintf("%s-%s-%d", from, ex.appchainDID, begin)
		var (
			ibtp *pb.IBTP
			err  error
		)
		retry.Retry(func(attempt uint) error {
			ibtp, err = ex.syncer.QueryIBTP(ibtpID)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp %s: %s", ibtpID, err.Error())
				return fmt.Errorf("fetch ibtp %s: %w", ibtpID, err)
			}
			return nil
		}, strategy.Wait(1*time.Second))
		entry := ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()})
		ex.handleIBTP(ibtp, entry)
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

		retry.Retry(func(attempt uint) error {
			original, err := ex.queryIBTP(from, begin)
			if err != nil {
				return err
			}
			receipt, err := ex.exec.QueryIBTPReceipt(original)
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
			return nil
		}, strategy.Wait(500*time.Millisecond))
	}
	return nil
}

func (ex *Exchanger) updateInterchainMeta() {
	updatedMeta := ex.syncer.QueryInterchainMeta()
	ex.interchainCounter = updatedMeta.InterchainCounter
	ex.logger.Info("Update interchain meta from bitxhub")
}

func (ex *Exchanger) updateSourceReceiptMeta() {
	updatedMeta := ex.syncer.QueryInterchainMeta()
	ex.sourceReceiptCounter = updatedMeta.SourceReceiptCounter
	ex.logger.Info("Update sourceReceiptCounter meta from bitxhub")
}
