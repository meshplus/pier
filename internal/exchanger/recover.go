package exchanger

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/hexutil"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/adapt/appchain_adapter"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) handleMissingIBTPByServicePair(begin, end uint64, fromAdapt, toAdapt adapt.Adapt, srcService, targetService string, isReq bool) {
	adaptName := fromAdapt.Name()
	for ; begin <= end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": fmt.Sprintf("%s-%s", srcService, targetService),
			"index":        begin,
			"end":          end,
			"isReq":        isReq,
		}).Info("handle missing event from:" + adaptName)
		ibtp := ex.queryIBTP(fromAdapt, fmt.Sprintf("%s-%s-%d", srcService, targetService, begin), isReq)
		//transaction timeout rollback in direct mode
		if strings.EqualFold(ex.mode, repo.DirectMode) {
			if isRollback := ex.isIBTPRollbackForDirect(ibtp); isRollback {
				// src pier notify src and dest rollback, ibtp should always be interchain
				if isReq == false {
					isReq = true
					ibtp = ex.queryIBTP(ex.srcAdapt, fmt.Sprintf("%s-%s-%d", srcService, targetService, begin), isReq)
				}
				ex.rollbackIBTPForDirect(ibtp)
				return
			}
		}
		ex.sendIBTP(fromAdapt, toAdapt, ibtp)
	}
}

func (ex *Exchanger) sendIBTP(srcAdapt, destAdapt adapt.Adapt, ibtp *pb.IBTP) {
	adaptName := destAdapt.Name()
	if err := retry.Retry(func(attempt uint) error {
		if err := destAdapt.SendIBTP(ibtp); err != nil {
			ex.logger.Warnf("send IBTP %s to Adapt %s: %s", ibtp.ID(), adaptName, err.Error())
			if err, ok := err.(*adapt.SendIbtpError); ok {
				if err.NeedRetry() {
					ibtp = ex.queryIBTP(srcAdapt, ibtp.ID(), ibtp.Category() == pb.IBTP_REQUEST)
					return fmt.Errorf("retry sending ibtp")
				}
			}
		}
		return nil
	}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
		ex.logger.Panic(err)
	}
}

func (ex *Exchanger) recover(srcServiceMeta map[string]*pb.Interchain, destServiceMeta map[string]*pb.Interchain) {
	// handle src -> dest
	ex.logger.Info("Start To Recover IBTPs!")
	for _, interchain := range srcServiceMeta {
		for k, count := range interchain.InterchainCounter {
			var destCount uint64
			if _, ok := destServiceMeta[interchain.ID]; !ok {
				destServiceMeta[interchain.ID] = &pb.Interchain{
					InterchainCounter:       make(map[string]uint64, 0),
					ReceiptCounter:          make(map[string]uint64, 0),
					SourceInterchainCounter: make(map[string]uint64, 0),
					SourceReceiptCounter:    make(map[string]uint64, 0),
				}
				destCount = 0
			} else {
				destCount, ok = destServiceMeta[interchain.ID].InterchainCounter[k]
				if !ok {
					destCount = 0
				}
			}

			// handle the situation that dst chain rollback failed but interchainCounter is balanced
			if ex.mode == repo.DirectMode && destCount == count {
				var begin uint64
				for begin = 1; begin <= count; begin++ {
					IBTPid := fmt.Sprintf("%s-%s-%d", interchain.ID, k, begin)
					_, _, txStatus, err := ex.srcAdapt.(*appchain_adapter.AppchainAdapter).GetDirectTransactionMeta(IBTPid)
					if err != nil {
						ex.logger.Panicf("fail to get direct transaction status for ibtp %s", IBTPid)
					}
					if txStatus == 2 { // transaction status is begin_rollback
						// notify dst chain rollback
						ibtp := ex.queryIBTP(ex.srcAdapt, IBTPid, true)
						ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
						ex.sendIBTPForDirect(ex.srcAdapt, ex.destAdapt, ibtp, ex.isIBTPBelongSrc(ibtp), true)
					}
				}
			}

			// Some Interchain from interchain.ID to k have not reached the bxh
			// handle unsentIBTP : query IBTP -> sendIBTP
			if destCount < count {
				ex.handleMissingIBTPByServicePair(destCount+1, count, ex.srcAdapt, ex.destAdapt, interchain.ID, k, true)
				// success then equal index
				destServiceMeta[interchain.ID].InterchainCounter[k] = count
			}
		}

		// Some Receipt from k to interchain.ID have not reached the bxh
		for k, count := range interchain.SourceReceiptCounter {
			var destCount uint64
			if _, ok := destServiceMeta[interchain.ID]; !ok {
				destServiceMeta[interchain.ID] = &pb.Interchain{
					InterchainCounter:       make(map[string]uint64, 0),
					ReceiptCounter:          make(map[string]uint64, 0),
					SourceInterchainCounter: make(map[string]uint64, 0),
					SourceReceiptCounter:    make(map[string]uint64, 0),
				}
				destCount = 0
			} else {
				destCount, ok = destServiceMeta[interchain.ID].SourceReceiptCounter[k]
				if !ok {
					destCount = 0
				}
			}
			// handle unsentIBTP : query IBTP -> sendIBTP
			if destCount < count {
				ex.handleMissingIBTPByServicePair(destCount+1, count, ex.srcAdapt, ex.destAdapt, k, interchain.ID, false)
				destServiceMeta[interchain.ID].SourceReceiptCounter[k] = count
			}
		}
	}

	// handle dest -> src
	for _, interchain := range destServiceMeta {
		for k, count := range interchain.SourceInterchainCounter {
			var srcCount uint64
			if _, ok := srcServiceMeta[interchain.ID]; !ok {
				srcServiceMeta[interchain.ID] = &pb.Interchain{
					InterchainCounter:       make(map[string]uint64, 0),
					ReceiptCounter:          make(map[string]uint64, 0),
					SourceInterchainCounter: make(map[string]uint64, 0),
					SourceReceiptCounter:    make(map[string]uint64, 0),
				}
			} else {
				srcCount, ok = srcServiceMeta[interchain.ID].SourceInterchainCounter[k]
				if !ok {
					srcCount = 0
				}
			}

			// srcCount means the count of Interchain from k to interchain.ID
			// handle unsentIBTP : query IBTP -> sendIBTP
			if srcCount < count {
				if ex.mode == repo.RelayMode {
					ex.logger.WithFields(logrus.Fields{"from": interchain.ID, "to": k, "current dest chain Count": srcCount}).
						Info("start init pool when interchain have not reached the dest appchain")
					ex.destAdapt.InitIbtpPool(interchain.ID, k, pb.IBTP_REQUEST, srcCount)
				}
				ex.handleMissingIBTPByServicePair(srcCount+1, count, ex.destAdapt, ex.srcAdapt, k, interchain.ID, true)
				srcServiceMeta[interchain.ID].SourceInterchainCounter[k] = count
			}
		}

		for k, count := range interchain.ReceiptCounter {
			var srcCount uint64
			if _, ok := srcServiceMeta[interchain.ID]; !ok {
				srcServiceMeta[interchain.ID] = &pb.Interchain{
					InterchainCounter:       make(map[string]uint64, 0),
					ReceiptCounter:          make(map[string]uint64, 0),
					SourceInterchainCounter: make(map[string]uint64, 0),
					SourceReceiptCounter:    make(map[string]uint64, 0),
				}
			} else {
				srcCount = srcServiceMeta[interchain.ID].ReceiptCounter[k]
				if !ok {
					srcCount = 0
				}
			}

			// handle unsentIBTP : query IBTP -> sendIBTP
			if srcCount < count {
				// if Some receipt from k to interchain.ID have not reached the src appchain,
				// need handleMissing receipt from index: srcCount+1,
				// so src chain's pier bxh adapter need init pool to ensure handle IBTP receipt correctly before handleMissing
				if ex.mode == repo.RelayMode {
					ex.logger.WithFields(logrus.Fields{"from": interchain.ID, "to": k, "current bxh Count": count}).
						Info("start init pool when interchain have not reached the bxh")
					ex.destAdapt.InitIbtpPool(k, interchain.ID, pb.IBTP_RESPONSE, srcCount)
				}
				ex.handleMissingIBTPByServicePair(srcCount+1, count, ex.destAdapt, ex.srcAdapt, interchain.ID, k, false)
				srcServiceMeta[interchain.ID].ReceiptCounter[k] = count
			}
		}
	}

	if ex.mode == repo.RelayMode {
		for serviceID, interchain := range destServiceMeta {
			// deal with source appchain rollback
			for k, interchainCounter := range interchain.InterchainCounter {
				var receiptCounter uint64
				if _, ok := srcServiceMeta[interchain.ID]; !ok {
					srcServiceMeta[interchain.ID] = &pb.Interchain{
						InterchainCounter:       make(map[string]uint64, 0),
						ReceiptCounter:          make(map[string]uint64, 0),
						SourceInterchainCounter: make(map[string]uint64, 0),
						SourceReceiptCounter:    make(map[string]uint64, 0),
					}
				} else {
					receiptCounter, ok = srcServiceMeta[serviceID].ReceiptCounter[k]
					if !ok {
						receiptCounter = 0
					}
				}

				ex.logger.Infof("check txStatus for service pair %s-%s from %d to %d for rollback", serviceID, k, receiptCounter+1, interchainCounter)
				for i := receiptCounter + 1; i <= interchainCounter; i++ {
					id := fmt.Sprintf("%s-%s-%d", serviceID, k, i)
					ibtp := ex.queryIBTP(ex.destAdapt, id, true)
					bxhProof := &pb.BxhProof{}
					if err := bxhProof.Unmarshal(ibtp.Proof); err != nil {
						ex.logger.Panicf("fail to unmarshal proof %s for ibtp %s", hexutil.Encode(ibtp.Proof), ibtp.ID())
					}

					if bxhProof.TxStatus == pb.TransactionStatus_BEGIN_FAILURE || bxhProof.TxStatus == pb.TransactionStatus_BEGIN_ROLLBACK {
						ex.logger.Infof("ibtp %s txStatus is %v, will rollback", ibtp.ID(), bxhProof.TxStatus)
						ex.sendIBTP(ex.destAdapt, ex.srcAdapt, ibtp)
					}
				}
			}
			_, chainID, _, err := ParseFullServiceID(serviceID)
			if err != nil {
				ex.logger.Errorf("ParseFullServiceID err:%s", err)
			}

			ex.logger.WithFields(logrus.Fields{"srcChainId": ex.srcChainId, "serviceID": serviceID, "interchain": interchain}).Info("start init pool")
			if chainID == ex.srcChainId {
				// init ibtp pool
				ex.initInterchainWrapperPool(interchain)
			}
		}
	}

	ex.logger.Info("End To Recover IBTPs!")
}

func (ex *Exchanger) initInterchainWrapperPool(interchain *pb.Interchain) {
	for k, srcInterchainCounter := range interchain.SourceInterchainCounter {
		ex.destAdapt.InitIbtpPool(k, interchain.ID, pb.IBTP_REQUEST, srcInterchainCounter)
	}

	for k, receiptCounter := range interchain.ReceiptCounter {
		ex.destAdapt.InitIbtpPool(interchain.ID, k, pb.IBTP_RESPONSE, receiptCounter)
	}
}

func (ex *Exchanger) recoverUnion(srcServiceMeta map[string]*pb.Interchain, destServiceMeta map[string]*pb.Interchain) {
	// handle src -> dest
	ex.logger.Info("Start To Recover IBTPs!")
	for _, interchain := range destServiceMeta {
		for k, count := range interchain.InterchainCounter {
			destCount, ok := srcServiceMeta[interchain.ID].InterchainCounter[k]
			if !ok {
				destCount = 0
			}
			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.destAdapt, ex.srcAdapt, interchain.ID, k, true)
			// success then equal index
			srcServiceMeta[interchain.ID].InterchainCounter[k] = count
		}
		for k, count := range interchain.SourceReceiptCounter {
			destCount, ok := srcServiceMeta[interchain.ID].SourceReceiptCounter[k]
			if !ok {
				destCount = 0
			}
			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.destAdapt, ex.srcAdapt, k, interchain.ID, false)
			srcServiceMeta[interchain.ID].SourceReceiptCounter[k] = count
		}
	}

	// handle dest -> src
	for _, interchain := range srcServiceMeta {
		for k, count := range interchain.SourceInterchainCounter {
			destCount, ok := destServiceMeta[interchain.ID].SourceInterchainCounter[k]
			if !ok {
				destCount = 0
			}

			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.srcAdapt, ex.destAdapt, k, interchain.ID, true)
			destServiceMeta[interchain.ID].SourceInterchainCounter[k] = count
		}

		for k, count := range interchain.ReceiptCounter {
			destCount, ok := destServiceMeta[interchain.ID].ReceiptCounter[k]
			if !ok {
				destCount = 0
			}

			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.srcAdapt, ex.destAdapt, interchain.ID, k, false)
			destServiceMeta[interchain.ID].ReceiptCounter[k] = count
		}
	}
	ex.logger.Info("End To Recover IBTPs!")
}
