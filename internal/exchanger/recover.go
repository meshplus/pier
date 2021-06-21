package exchanger

import (
	"fmt"
	"math/big"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) handleMissingLockFromMnt(rAppchainIndex int64, aAppchainIndex int64) error {
	if rAppchainIndex < aAppchainIndex {
		for index := rAppchainIndex + 1; index < aAppchainIndex+1; index++ {
			// query missing lock event
			lockEvent := ex.exec.QueryLockEventByIndex(index)
			ex.logger.Info("Receive lock event from monitor")
			// syner mint
			err := ex.syncer.SendLockEvent(lockEvent)
			if err != nil {
				return err
			}
			ex.rAppchainIndex++
		}
	}
	return nil
}

func (ex *Exchanger) handleMissingBurnFromSyncer(aRelayIndex int64, rRelayIndex int64) error {
	if aRelayIndex < rRelayIndex {
		for index := aRelayIndex + 1; index < rRelayIndex+1; index++ {
			// query missing burn event
			burnEvent := ex.syncer.QueryBurnEventByIndex(index)
			ex.logger.Info("Receive burn event from syner")
			// syner unlock
			err := ex.exec.SendBurnEvent(burnEvent)
			if err != nil {
				return err
			}
			ex.aRelayIndex++
		}
	}
	return nil
}

func (ex *Exchanger) recoverMintAndBurnRelay() {
	ex.rRelayIndex = ex.syncer.JsonrpcClient().RelayIndex()
	ex.rAppchainIndex = ex.syncer.JsonrpcClient().AppchainIndex()
	ex.aRelayIndex = ex.exec.QueryRelayIndex()
	ex.aAppchainIndex = ex.exec.QueryAppchainIndex()
	height, _ := ex.syncer.JsonrpcClient().InterchainSwapSession().Index2Height(big.NewInt(ex.rRelayIndex))
	ex.rIndex2Height = height.Int64()
	ex.aIndex2Height = ex.exec.QueryFilterLockStart(ex.aAppchainIndex)

	// deal missing lock event
	ex.handleMissingLockFromMnt(ex.rAppchainIndex, ex.aAppchainIndex)
	// deal missing burn event
	ex.handleMissingBurnFromSyncer(ex.aRelayIndex, ex.rRelayIndex)
	go ex.listenBurnEventFromSyncer()
}

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
		entry := ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()})
		ex.handleIBTP(&model.WrappedIBTP{Ibtp: ibtp, IsValid: isValid}, entry)
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
		entry.Info("Send missing receipt to bitxhub")

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
