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
	srcRollbackMeta := ex.mnt.QuerySrcRollbackMeta()
	for to, idx := range mntMeta {
		beginIndex, ok := ex.interchainCounter[to]
		if !ok {
			beginIndex = 0
		}

		rollbackIndex, ok := srcRollbackMeta[to]
		if !ok {
			rollbackIndex = 0
		}

		// the appchain didn't send ibtp because of network issue, and the interchain tx has already been rollbacked
		// in src appchain, so need to set interchain index on bitxhub to avoid index omitting
		if err := ex.handleRollbackedIBTP(to, beginIndex+1, rollbackIndex+1); err != nil {
			ex.logger.WithFields(logrus.Fields{
				"address": to,
				"error":   err.Error(),
			}).Panic("Send rollbacked ibtp")
		}

		if rollbackIndex > beginIndex {
			beginIndex = rollbackIndex
			ex.interchainCounter[to] = beginIndex
		}

		if err := ex.handleMissingIBTPFromMnt(to, beginIndex+1, idx+1); err != nil {
			ex.logger.WithFields(logrus.Fields{"address": to, "error": err.Error()}).Panic("Get missing event from contract")
		}
	}

	// recover unsent receipt to counterpart chain
	execMeta := ex.exec.QueryInterchainMeta()
	for from, idx := range execMeta {
		curIdx, ok := ex.sourceReceiptCounter[from]
		if curIdx >= idx {
			continue
		}
		beginIndex := curIdx + 1
		if !ok {
			beginIndex = 1
		}

		// Rollback the last receipt on bitxhub if it is not rollbacked on dst appchain
		ibtpId := fmt.Sprintf("%s-%s-%d", from, ex.pierID, beginIndex)
		status := pb.TransactionStatus_BEGIN
		if err := retry.Retry(func(attempt uint) error {
			stat, err := ex.syncer.GetTxStatus(ibtpId)
			if err != nil {
				ex.logger.WithFields(logrus.Fields{
					"error": err,
					"id":    ibtpId,
				}).Error("Get tx status")
				return err
			}

			status = stat

			return nil
		}, strategy.Wait(2*time.Second)); err != nil {
			ex.logger.WithFields(logrus.Fields{
				"id":    ibtpId,
				"error": err,
			}).Errorf("Retry to get tx status")
		}

		if status == pb.TransactionStatus_ROLLBACK {
			dstRollbackMeta := ex.exec.QueryDstRollbackMeta()
			if dstRollbackMeta[from] < beginIndex {
				var ibtp *pb.IBTP
				if err := retry.Retry(func(attempt uint) error {
					original, _, err := ex.queryIBTP(ibtpId, from)
					if err != nil {
						ex.logger.WithFields(logrus.Fields{
							"id":    ibtpId,
							"error": err,
						}).Errorf("Retry to get query ibtp")
						return err
					}

					ibtp = original

					return nil
				}, strategy.Wait(2*time.Second)); err != nil {
					ex.logger.WithFields(logrus.Fields{
						"id":    ibtpId,
						"error": err,
					}).Panic("Retry to query IBTP")
				}
				ex.exec.Rollback(ibtp, false)
				ex.logger.WithFields(logrus.Fields{
					"id": ibtp.ID(),
				}).Info("Rollback ibtp")
			}
		}

		if err := ex.handleMissingReceipt(from, beginIndex, idx+1); err != nil {
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

func (ex *Exchanger) handleRollbackedIBTP(to string, begin, end uint64) error {
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"to":    to,
			"index": begin,
		}).Info("Send Rollbacked IBTP")

		ibtp, err := ex.mnt.QueryIBTP(fmt.Sprintf("%s-%s-%d", ex.pierID, to, begin))
		if err != nil {
			return fmt.Errorf("fetch ibtp:%w", err)
		}

		ibtp.Type = pb.IBTP_ROLLBACK

		if err := ex.sendIBTP(ibtp); err != nil {
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

func (ex *Exchanger) handleMissingCallback(to string, begin, end uint64) error {
	ex.logger.WithFields(logrus.Fields{
		"to":    to,
		"begin": begin,
		"end":   end,
	}).Info("Get missing callbacks from bitxhub")
	for ; begin < end; begin++ {
		if err := retry.Retry(func(attempt uint) error {
			ibtp, isValid, err := ex.queryIBTP(fmt.Sprintf("%s-%s-%d", ex.pierID, to, begin), to)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp: %s", err.Error())
				return err
			}
			// if this ibtp is not valid, try to rollback
			if !isValid {
				ex.logger.WithFields(logrus.Fields{
					"to":    to,
					"index": begin,
				}).Info("Rollback missing callbacks from bitxhub")
				ex.feedIBTPReceipt(&model.WrappedIBTP{Ibtp: ibtp, IsValid: false})
			}
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
		var (
			ibtp    *pb.IBTP
			isValid bool
			err     error
		)
		ibtpID := fmt.Sprintf("%s-%s-%d", from, ex.pierID, begin)
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
	entry := ex.logger.WithFields(logrus.Fields{
		"from": from,
	})

	for ; begin < end; begin++ {
		entry = entry.WithFields(logrus.Fields{
			"index": begin,
		})
		entry.Info("Send missing receipt to bitxhub")
		retry.Retry(func(attempt uint) error {
			original, _, err := ex.queryIBTP(fmt.Sprintf("%s-%s-%d", from, ex.pierID, begin), from)
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
