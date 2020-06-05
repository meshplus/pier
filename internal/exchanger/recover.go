package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) recoverRelay() {
	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryLatestMeta()
	for to, idx := range mntMeta {
		beginIndex, ok := ex.interchainCounter[to]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingIBTP(to, beginIndex+1, idx+1); err != nil {
			logger.WithFields(logrus.Fields{
				"address": to,
				"error":   err.Error(),
			}).Panic("Get missing receipt from contract")
		}
	}

	// recover unsent receipt to counterpart chain
	execMeta := ex.exec.QueryLatestMeta()
	for from, idx := range execMeta {
		beginIndex, ok := ex.sourceReceiptMeta[from]
		if !ok {
			beginIndex = 0
		}

		if err := ex.handleMissingReceipt(from, beginIndex+1, idx+1); err != nil {
			logger.WithFields(logrus.Fields{
				"address": from,
				"error":   err.Error(),
			}).Panic("Get missing receipt from contract")
		}
	}
}

func (ex *Exchanger) recoverDirect(dstPierID string, interchainIndex uint64, receiptIndex uint64) {
	// recover unsent interchain ibtp
	mntMeta := ex.mnt.QueryLatestMeta()
	index, ok := mntMeta[dstPierID]
	if !ok {
		logger.Infof("Appchain %s not exist", dstPierID)
		return
	}
	if err := ex.handleMissingIBTP(dstPierID, interchainIndex+1, index+1); err != nil {
		logger.WithFields(logrus.Fields{
			"address": dstPierID,
			"error":   err.Error(),
		}).Error("Handle missing ibtp")
	}

	// recoverDirect unsent receipt to counterpart chain
	execMeta := ex.exec.QueryLatestMeta()
	index, ok = execMeta[dstPierID]
	if !ok {
		return
	}
	if err := ex.handleMissingReceipt(dstPierID, receiptIndex+1, index+1); err != nil {
		logger.WithFields(logrus.Fields{
			"address": dstPierID,
			"error":   err.Error(),
		}).Panic("Get missing receipt from contract")
	}
}

func (ex *Exchanger) handleMissingIBTP(to string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		logger.WithFields(logrus.Fields{
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
	}

	return nil
}

func (ex *Exchanger) handleMissingReceipt(from string, begin uint64, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing receipt is required >= 1")
	}

	entry := logger.WithFields(logrus.Fields{
		"from": from,
	})

	for ; begin < end; begin++ {
		err := retry.Retry(func(attempt uint) error {
			entry = entry.WithFields(logrus.Fields{
				"index": begin,
			})
			original, err := ex.queryIBTP(from, begin)
			if err != nil {
				return err
			}

			receipt, err := ex.exec.QueryReceipt(from, begin, original)
			if err != nil {
				entry.WithField("error", err).Error("Get missing execution receipt")
				return err
			}

			// send receipt back to counterpart chain
			if err := ex.sendIBTP(receipt); err != nil {
				entry.WithField("error", err).Error("Send execution receipt to counterpart chain")
				return err
			}

			return nil
		}, strategy.Wait(1*time.Second))

		if err != nil {
			logger.Error(err)
			return err
		}

	}
	return nil
}
