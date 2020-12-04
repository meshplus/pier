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
	mntMeta := ex.mnt.QueryLatestMeta()
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
			logger.WithFields(logrus.Fields{
				"address": to,
				"error":   err.Error(),
			}).Panic("Send rollbacked ibtp")
		}

		if rollbackIndex > beginIndex {
			beginIndex = rollbackIndex
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
			beginIndex = 1
		}

		// Rollback the last receipt on bitxhub if it is not rollbacked on dst appchain
		ibtpId := fmt.Sprintf("%s-%s-%d", from, ex.pierID, beginIndex)
		status := pb.TransactionStatus_BEGIN
		if err := retry.Retry(func(attempt uint) error {
			stat, err := ex.agent.GetTxStatus(ibtpId)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"error": err,
					"id":    ibtpId,
				}).Error("Get tx status")
				return err
			}

			status = stat

			return nil
		}, strategy.Wait(2*time.Second)); err != nil {
			logger.WithFields(logrus.Fields{
				"id":    ibtpId,
				"error": err,
			}).Errorf("Retry to get tx status")
		}

		if status == pb.TransactionStatus_ROLLBACK {
			dstRollbackMeta := ex.exec.QueryDstRollbackMeta()
			if dstRollbackMeta[from] < beginIndex {
				var ibtp *pb.IBTP
				if err := retry.Retry(func(attempt uint) error {
					original, err := ex.queryIBTP(from, beginIndex)
					if err != nil {
						logger.WithFields(logrus.Fields{
							"id":    ibtpId,
							"error": err,
						}).Errorf("Retry to get tx status")
						return err
					}

					ibtp = original

					return nil
				}, strategy.Wait(2*time.Second)); err != nil {
					logger.WithFields(logrus.Fields{
						"id":    ibtpId,
						"error": err,
					}).Panic("Retry to query IBTP")
				}
				ex.exec.Rollback(ibtp, false)
				logger.WithFields(logrus.Fields{
					"id": ibtp.ID(),
				}).Info("Rollback ibtp")
			}
		}

		if err := ex.handleMissingReceipt(from, beginIndex, idx+1); err != nil {
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

func (ex *Exchanger) handleRollbackedIBTP(to string, begin, end uint64) error {
	for ; begin < end; begin++ {
		logger.WithFields(logrus.Fields{
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
