package exchanger

import (
	"fmt"
	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/hexutil"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/adapt/appchain_adapter"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

func (ex *Exchanger) handleMissingIBTPByServicePair(begin, end uint64, fromAdapt, toAdapt adapt.Adapt, srcService, targetService string, isReq bool) {
	adaptName := fromAdapt.Name()
	for ; begin <= end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": fmt.Sprintf("%s-%s", srcService, targetService),
			"index":        begin,
			"isReq":        isReq,
		}).Info("handle missing event from:" + adaptName)
		ibtp := ex.queryIBTP(fromAdapt, fmt.Sprintf("%s-%s-%d", srcService, targetService, begin), isReq)
		//transaction timeout rollback in direct mode
		if strings.EqualFold(ex.mode, repo.DirectMode) {
			if isRollback := ex.isIBTPRollbackForDirect(ibtp); isRollback {
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
			destCount, ok := destServiceMeta[interchain.ID].InterchainCounter[k]
			if !ok {
				destCount = 0
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
			destCount, ok := destServiceMeta[interchain.ID].SourceReceiptCounter[k]
			if !ok {
				destCount = 0
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
			srcCount := srcServiceMeta[interchain.ID].SourceInterchainCounter[k]

			// srcCount means the count of Interchain from k to interchain.ID
			// handle unsentIBTP : query IBTP -> sendIBTP
			if srcCount < count {
				ex.handleMissingIBTPByServicePair(srcCount+1, count, ex.destAdapt, ex.srcAdapt, k, interchain.ID, true)
				srcServiceMeta[interchain.ID].SourceInterchainCounter[k] = count
			}
		}

		for k, count := range interchain.ReceiptCounter {
			srcCount := srcServiceMeta[interchain.ID].ReceiptCounter[k]

			// handle unsentIBTP : query IBTP -> sendIBTP
			if srcCount < count {
				ex.handleMissingIBTPByServicePair(srcCount+1, count, ex.destAdapt, ex.srcAdapt, interchain.ID, k, false)
				srcServiceMeta[interchain.ID].ReceiptCounter[k] = count
			}
		}
	}

	if ex.mode == repo.RelayMode {
		for serviceID, interchain := range destServiceMeta {
			// deal with source appchain rollback
			for k, interchainCounter := range interchain.InterchainCounter {
				receiptCounter := srcServiceMeta[serviceID].ReceiptCounter[k]

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
			// init ibtp pool
			ex.initInterchainWrapperPool(interchain)
		}
	}

	ex.logger.Info("End To Recover IBTPs!")
}

func (ex *Exchanger) initInterchainWrapperPool(interchain *pb.Interchain) {
	for k, srcInterchainCounter := range interchain.SourceInterchainCounter {
		ex.destAdapt.InitIbtpPool(k, interchain.ID, pb.IBTP_REQUEST, srcInterchainCounter)
	}

	for k, srcReceipCounter := range interchain.SourceReceiptCounter {
		ex.destAdapt.InitIbtpPool(k, interchain.ID, pb.IBTP_RESPONSE, srcReceipCounter)
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
