package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/utils"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) recoverRelay() {
	ex.handleUnsentIBTP()

	ex.handleUnsentIBTPReceipt()

	ex.handleUnreceivedIBTP()

	ex.handleUnreceivedIBTPReceipt()
}

func (ex *Exchanger) handleUnsentIBTP() {
	mntMeta := ex.mnt.QueryOuterMeta()
	callbackMeta := ex.exec.QueryCallbackMeta()

	// servicePair format: src full service ID-dst full service ID
	for servicePair, idx := range mntMeta {
		var (
			beginIndex    = uint64(0)
			rollbackIndex = uint64(0)
		)

		srcServiceID, dstServiceID, err := utils.ParseServicePair(servicePair)
		if err != nil {
			ex.logger.Panicf("parse service pair when sending out IBTP: %v", err)
		}
		ic, ok := ex.serviceMeta[srcServiceID]
		if ok {
			beginIndex = ic.InterchainCounter[dstServiceID]
		} else {
			ic = &pb.Interchain{
				ID:                      dstServiceID,
				InterchainCounter:       make(map[string]uint64),
				ReceiptCounter:          make(map[string]uint64),
				SourceInterchainCounter: make(map[string]uint64),
				SourceReceiptCounter:    make(map[string]uint64),
			}
		}

		rollbackIndex = callbackMeta[servicePair]

		// the appchain didn't send ibtp because of network issue, and the interchain tx has already been rollbacked
		// in src appchain, so need to set interchain index on bitxhub to avoid index omitting
		if err := ex.handleRollbackedIBTP(servicePair, beginIndex+1, rollbackIndex+1); err != nil {
			ex.logger.WithFields(logrus.Fields{
				"service pair": servicePair,
				"error":        err.Error(),
			}).Panic("Send rollbacked ibtp to bitxhub")
		}

		if rollbackIndex > beginIndex {
			beginIndex = rollbackIndex
			ic.InterchainCounter[dstServiceID] = beginIndex
		}

		if beginIndex < idx {
			if err := ex.handleMissingIBTPFromMnt(servicePair, beginIndex+1, idx+1); err != nil {
				ex.logger.WithFields(logrus.Fields{
					"service pair": servicePair,
					"error":        err.Error(),
				}).Panic("Handle unsent ibtp to bitxhub")
			}
			ic.InterchainCounter[dstServiceID] = idx
		}
	}
}

func (ex *Exchanger) handleUnsentIBTPReceipt() {
	inMeta := ex.exec.QueryInterchainMeta()

	for serviceID, meta := range ex.serviceMeta {
		for srcServiceID := range meta.SourceInterchainCounter {
			servicePair := fmt.Sprintf("%s-%s", srcServiceID, serviceID)

			beginIndex := meta.SourceReceiptCounter[srcServiceID]
			idx := inMeta[servicePair]

			if beginIndex > 0 {
				// Rollback the last receipt on bitxhub if it is not rollbacked on dst appchain
				ibtpId := fmt.Sprintf("%s-%d", servicePair, beginIndex)
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
					if dstRollbackMeta[servicePair] < beginIndex {
						var ibtp *pb.IBTP
						_, srcAppchainID, _, err := utils.ParseFullServiceID(srcServiceID)
						if err != nil {
							ex.logger.Panic(err)
						}

						if err := retry.Retry(func(attempt uint) error {
							original, _, err := ex.queryIBTP(ibtpId, srcAppchainID, true)
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
			}

			if idx > beginIndex {
				if err := ex.handleMissingReceipt(servicePair, beginIndex+1, idx+1); err != nil {
					ex.logger.WithFields(logrus.Fields{
						"service pair": servicePair,
						"error":        err.Error(),
					}).Panic("Get missing callbacks from bitxhub")
				}
				meta.SourceReceiptCounter[srcServiceID] = idx
			}
		}
	}
}

func (ex *Exchanger) handleUnreceivedIBTP() {
	inMeta := ex.exec.QueryInterchainMeta()

	for serviceID, meta := range ex.serviceMeta {
		for srcServiceID, idx := range meta.SourceInterchainCounter {
			servicePair := fmt.Sprintf("%s-%s", srcServiceID, serviceID)

			beginIndex := inMeta[servicePair]

			if err := ex.handleMissingIBTPFromSyncer(servicePair, beginIndex+1, idx+1); err != nil {
				ex.logger.WithFields(logrus.Fields{
					"service pair": servicePair,
					"error":        err.Error(),
				}).Panic("Get missing callbacks from bitxhub")
			}
		}
	}
}

func (ex *Exchanger) handleUnreceivedIBTPReceipt() {
	for serviceID, meta := range ex.serviceMeta {
		for dstServiceID, idx := range meta.ReceiptCounter {
			servicePair := fmt.Sprintf("%s-%s", serviceID, dstServiceID)

			beginIndex := ex.callbackMeta[servicePair]

			if beginIndex < idx {
				if err := ex.handleMissingCallback(servicePair, beginIndex+1, idx+1); err != nil {
					ex.logger.WithFields(logrus.Fields{
						"service pair": servicePair,
						"error":        err.Error(),
					}).Panic("Get missing callbacks from bitxhub")
				}
				ex.callbackMeta[servicePair] = idx
				ex.logger.Warnf("handleUnreceivedIBTPReceipt for %s-%d", servicePair, idx)
			}
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

func (ex *Exchanger) handleRollbackedIBTP(servicePair string, begin, end uint64) error {
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": servicePair,
			"index":        begin,
		}).Info("Send Rollbacked IBTP")

		ibtp, err := ex.mnt.QueryIBTP(fmt.Sprintf("%s-%d", servicePair, begin))
		if err != nil {
			return fmt.Errorf("fetch ibtp:%w", err)
		}

		// todo: Temporary annotation
		//ibtp.Type = pb.IBTP_ROLLBACK

		if err := ex.sendIBTP(ibtp); err != nil {
			return err
		}
	}

	return nil
}

func (ex *Exchanger) handleMissingCallback(servicePair string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing callbacks is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": servicePair,
			"index":        begin,
		}).Info("Get missing callbacks from bitxhub")

		_, dstServiceID, err := utils.ParseServicePair(servicePair)
		if err != nil {
			return err
		}
		_, dstAppchainID, _, err := utils.ParseFullServiceID(dstServiceID)
		if err != nil {
			return err
		}

		if err := retry.Retry(func(attempt uint) error {
			ibtp, isValid, err := ex.queryIBTP(fmt.Sprintf("%s-%d", servicePair, begin), dstAppchainID, false)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp: %s", err.Error())
				return err
			}
			// if this ibtp is not valid, try to rollback
			entry := ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()})
			if isValid {
				ex.handleIBTP(&model.WrappedIBTP{Ibtp: ibtp, IsValid: isValid}, entry)
			} else {
				ibtp, err = ex.mnt.QueryIBTP(ibtp.ID())
				if err != nil {
					ex.logger.Errorf("query ibtp from monitor: %s", err.Error())
					return err
				}
				ex.exec.Rollback(ibtp, true)
			}

			return nil
		}, strategy.Wait(500*time.Millisecond), strategy.Limit(10)); err != nil {
			return err
		}
	}

	return nil
}

func (ex *Exchanger) handleMissingIBTPFromMnt(servicePair string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service paire": servicePair,
			"index":         begin,
		}).Info("Get missing event from contract")

		if err := retry.Retry(func(attempt uint) error {
			ibtp, err := ex.mnt.QueryIBTP(fmt.Sprintf("%s-%d", servicePair, begin))
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

func (ex *Exchanger) handleMissingIBTPFromSyncer(servicePair string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": servicePair,
			"index":        begin,
		}).Info("Get missing event from bitxhub")

		ibtpID := fmt.Sprintf("%s-%d", servicePair, begin)
		var (
			ibtp    *pb.IBTP
			isValid bool
			err     error
		)
		retry.Retry(func(attempt uint) error {
			ibtp, isValid, err = ex.syncer.QueryIBTP(ibtpID, true)
			if err != nil {
				ex.logger.Errorf("Fetch ibtp %s: %s", ibtpID, err.Error())
				return fmt.Errorf("fetch ibtp %s: %w", ibtpID, err)
			}
			return nil
		}, strategy.Wait(1*time.Second))
		entry := ex.logger.WithFields(logrus.Fields{"type": ibtp.Type, "id": ibtp.ID()})
		ex.handleIBTP(&model.WrappedIBTP{Ibtp: ibtp, IsValid: isValid}, entry)
	}

	return nil
}

func (ex *Exchanger) handleMissingReceipt(servicePair string, begin uint64, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing receipt is required >= 1")
	}

	entry := ex.logger.WithFields(logrus.Fields{
		"service pair": servicePair,
	})

	for ; begin < end; begin++ {
		entry = entry.WithFields(logrus.Fields{
			"index": begin,
		})
		entry.Info("Send missing receipt to bitxhub")

		srcServiceID, _, err := utils.ParseServicePair(servicePair)
		if err != nil {
			return err
		}
		_, srcAppchainID, _, err := utils.ParseFullServiceID(srcServiceID)
		if err != nil {
			return err
		}

		retry.Retry(func(attempt uint) error {
			original, _, err := ex.queryIBTP(fmt.Sprintf("%s-%d", servicePair, begin), srcAppchainID, true)
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
			return nil
		}, strategy.Wait(1*time.Second))
	}
	return nil
}

func (ex *Exchanger) recoverUnion() error {
	ex.bxhID = fmt.Sprintf("%d", ex.getBitXHubChainID())
	go func() {
		for {
			err := ex.router.Broadcast(ex.bxhID)
			if err != nil {
				ex.logger.Warnf("broadcast BitXHub ID %s: %w", ex.bxhID, err)
			}

			time.Sleep(time.Second)
		}
	}()

	outBxhServiceM := make(map[string][]string)
	bitxhubIDs, err := ex.syncer.GetBitXHubIDs()
	if err != nil {
		return err
	}
	ex.logger.Warnf("get bitxhub IDs %s", bitxhubIDs)
	for _, bitxhubID := range bitxhubIDs {
		outBxhServiceM[bitxhubID] = []string{}
	}

	serviceIDs, err := ex.syncer.GetServiceIDs()
	if err != nil {
		return err
	}

	for _, serviceID := range serviceIDs {
		interchain0 := ex.syncer.QueryInterchainMeta(serviceID)
		ex.logger.Infof("get service %s interchain from current bitxhub: %v", serviceID, interchain0)
		outServiceM, err := ex.getOutServices(interchain0)
		if err != nil {
			return err
		}

		for bxhID, outServices := range outServiceM {
			interchain1, err := ex.router.QueryInterchain(bxhID, serviceID)
			if err != nil {
				continue
			}

			for _, outService := range outServices {
				if interchain0.InterchainCounter[outService] > interchain1.InterchainCounter[outService] {
					ex.sendMissingIBTPToUnionPier(serviceID, outService, interchain1.InterchainCounter[outService]+1, interchain0.InterchainCounter[outService], true)
				}
				if interchain0.ReceiptCounter[outService] < interchain1.ReceiptCounter[outService] {
					ex.askMissingIBTPFromUnionPier(serviceID, outService, interchain0.ReceiptCounter[outService]+1, interchain1.ReceiptCounter[outService], false)
				}
			}
		}

		inServiceM, err := ex.getInServices(interchain0)
		if err != nil {
			return err
		}

		for bxhID, inServices := range inServiceM {
			interchain1, err := ex.router.QueryInterchain(bxhID, serviceID)
			if err != nil {
				continue
			}

			for _, inService := range inServices {
				if interchain0.SourceInterchainCounter[inService] < interchain1.SourceInterchainCounter[inService] {
					ex.askMissingIBTPFromUnionPier(inService, serviceID, interchain0.SourceInterchainCounter[inService]+1, interchain1.SourceInterchainCounter[inService], true)
				}
				if interchain0.SourceReceiptCounter[inService] > interchain1.SourceReceiptCounter[inService] {
					ex.sendMissingIBTPToUnionPier(inService, serviceID, interchain1.SourceReceiptCounter[inService]+1, interchain0.SourceReceiptCounter[inService], false)
				}
			}
		}
	}

	return nil
}

func (ex *Exchanger) updateInterchainMeta(serviceID string) {
	updatedMeta := ex.syncer.QueryInterchainMeta(serviceID)
	ex.serviceMeta[serviceID].InterchainCounter = updatedMeta.InterchainCounter
	ex.logger.Info("Update interchain meta from bitxhub")
}

func (ex *Exchanger) updateSourceReceiptMeta(serviceID string) {
	updatedMeta := ex.syncer.QueryInterchainMeta(serviceID)
	ex.serviceMeta[serviceID].SourceReceiptCounter = updatedMeta.SourceReceiptCounter
	ex.logger.Info("Update sourceReceiptCounter meta from bitxhub")
}

func (ex *Exchanger) getOutServices(interchain *pb.Interchain) (map[string][]string, error) {
	outServiceM := make(map[string][]string)
	for toService := range interchain.InterchainCounter {
		bxhID, _, _, err := utils.ParseFullServiceID(toService)
		if err != nil {
			return nil, err
		}
		if bxhID != ex.bxhID {
			outServiceM[bxhID] = append(outServiceM[bxhID], toService)
		}
	}

	return outServiceM, nil
}

func (ex *Exchanger) getInServices(interchain *pb.Interchain) (map[string][]string, error) {
	inServiceM := make(map[string][]string)
	for inService := range interchain.SourceInterchainCounter {
		bxhID, _, _, err := utils.ParseFullServiceID(inService)
		if err != nil {
			return nil, err
		}
		if bxhID != ex.bxhID {
			inServiceM[bxhID] = append(inServiceM[bxhID], inService)
		}
	}

	return inServiceM, nil
}

func (ex *Exchanger) sendMissingIBTPToUnionPier(from, to string, beginIdx, endIdx uint64, isReq bool) {
	for i := beginIdx; i <= endIdx; i++ {
		ibtpID := fmt.Sprintf("%s-%s-%d", from, to, i)
		ex.logger.Infof("send missing IBTP %s req %v to union pier", ibtpID, isReq)
		ibtp := ex.queryIBTPFromBitXHub(ibtpID, isReq)
		if err := ex.router.Route(ibtp); err != nil {
			ex.logger.Warnf("route ibtp %s type %s: %w", ibtpID, ibtp.Type.String(), err)
			return
		}
	}
}

func (ex *Exchanger) askMissingIBTPFromUnionPier(from, to string, beginIdx, endIdx uint64, isReq bool) {
	for i := beginIdx; i <= endIdx; i++ {
		ibtpID := fmt.Sprintf("%s-%s-%d", from, to, i)
		ex.logger.Infof("ask missing IBTP %s req %v from union pier", ibtpID, isReq)
		ibtp, err := ex.router.QueryIBTP(ibtpID, isReq)
		if err != nil {
			ex.logger.Warnf("query ibtp %s isReq %v: %w", ibtpID, isReq, err)
			return
		}
		if err := ex.syncer.SendIBTP(ibtp); err != nil {
			ex.logger.Warnf("send ibtp %s type %s: %w", ibtpID, ibtp.Type.String(), err)
			return
		}
	}
}

func (ex *Exchanger) queryIBTPFromBitXHub(id string, isReq bool) *pb.IBTP {
	var (
		ibtp  *pb.IBTP
		err   error
		signs []byte
	)

	if err := retry.Retry(func(attempt uint) error {
		ibtp, _, err = ex.syncer.QueryIBTP(id, isReq)
		return err
	}, strategy.Wait(time.Second*3)); err != nil {
		ex.logger.Panicf("query ibtp %s from bitxhub: %w", id, err)
	}

	if err := retry.Retry(func(attempt uint) error {
		var err error
		signs, err = ex.syncer.GetIBTPSigns(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		ex.logger.Panicf("get ibtp %s sign from bitxhub: %w", id, err)
	}

	ibtp.Proof = signs

	return ibtp
}
