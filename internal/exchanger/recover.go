package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) recoverRelay() {
	// recover possible unrollbacked ibtp
	callbackMeta := ex.exec.QueryCallbackMeta()
	for to, idx := range ex.interchainCounter {
		beginIndex, ok := callbackMeta[to]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingCallback(to, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{"address": to, "error": err.Error()}).Panic("Get missing callbacks from bitxhub")
		}
	}
	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryOuterMeta()
	for to, idx := range mntMeta {
		beginIndex, ok := ex.interchainCounter[to]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingIBTPFromMnt(to, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{"address": to, "error": err.Error()}).Panic("Get missing event from contract")
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

func (ex *Exchanger) recoverDirect(dstChainDID string, interchainIndex uint64, receiptIndex uint64) {
	ex.logger.WithFields(logrus.Fields{
		"dstChainDID":     dstChainDID,
		"interchainIndex": interchainIndex,
		"receiptIndex":    receiptIndex,
	}).Info("recover direct")

	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryOuterMeta()
	index := mntMeta[dstChainDID]
	ex.logger.Infof("handleMissingIBTPFromMnt for %s from %d to %d", dstChainDID, interchainIndex+1, index)
	if err := ex.handleMissingIBTPFromMnt(dstChainDID, interchainIndex+1, index+1); err != nil {
		ex.logger.WithFields(logrus.Fields{"address": dstChainDID, "error": err.Error()}).Error("Handle missing ibtp")
	}

	// recoverDirect unsent receipt to counterpart chain
	execMeta := ex.exec.QueryInterchainMeta()
	idx := execMeta[dstChainDID]
	ex.logger.Infof("handleMissingReceipt for %s from %d to %d", dstChainDID, receiptIndex+1, idx)
	if err := ex.handleMissingReceipt(dstChainDID, receiptIndex+1, idx+1); err != nil {
		ex.logger.WithFields(logrus.Fields{"address": dstChainDID, "error": err.Error()}).Panic("Get missing receipt from contract")
	}
}

func (ex *Exchanger) handleMissingCallback(to string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing callbacks is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"to":    to,
			"index": begin,
		}).Info("Get missing callbacks from bitxhub")

		if err := retry.Retry(func(attempt uint) error {
			ibtp, isValid, err := ex.queryIBTP(fmt.Sprintf("%s-%s-%d", ex.appchainDID, to, begin), to)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp: %s", err.Error())
				return err
			}
			// if this ibtp is not valid, try to rollback
			if !isValid {
				ex.feedIBTPReceipt(&model.WrappedIBTP{Ibtp: ibtp, IsValid: false})
			}
			return nil
		}, strategy.Wait(500*time.Millisecond), strategy.Limit(10)); err != nil {
			return err
		}
	}

	return nil
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

			return nil
		}, strategy.Wait(500*time.Millisecond)); err != nil {
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
			ibtp    *pb.IBTP
			isValid bool
			err     error
		)
		retry.Retry(func(attempt uint) error {
			ibtp, isValid, err = ex.syncer.QueryIBTP(ibtpID)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp %s: %s", ibtpID, err.Error())
				return fmt.Errorf("fetch ibtp %s: %w", ibtpID, err)
			}
			return nil
		}, strategy.Wait(1*time.Second))
		ex.handleIBTP(&model.WrappedIBTP{Ibtp: ibtp, IsValid: isValid})
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
		entry.Info("Send missing receipt")

		retry.Retry(func(attempt uint) error {
			original, _, err := ex.queryIBTP(fmt.Sprintf("%s-%s-%d", from, ex.appchainDID, begin), from)
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
		}, strategy.Wait(1*time.Second))
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
