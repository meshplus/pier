package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/adapt/appchain_adapter"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) listenIBTPFromDestAdaptForDirect(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromDestAdapt Stop!")
			return
		case ibtp, ok := <-ex.destIBTPMap[servicePair]:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.destAdaptName)
			index := ex.getCurrentIndexFromDest(ibtp)

			if index >= ibtp.Index {
				if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK {
					// dst chain receive rollback, need rollback
					ex.sendIBTPForDirect(ex.destAdapt, ex.srcAdapt, ibtp, !ex.isIBTPBelongSrc(ibtp), true)
				} else if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END {
					// receive rollback end from dst chain, src chain change transaction status
					ex.sendIBTPForDirect(ex.destAdapt, ex.srcAdapt, ibtp, !ex.isIBTPBelongSrc(ibtp), false)
				} else {
					ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				}
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.destAdapt, ex.srcAdapt, ibtp.From, ibtp.To, !ex.isIBTPBelongSrc(ibtp))
			}

			if isRollback := ex.isIBTPRollbackForDirect(ibtp); isRollback {
				// receipt time out, need update ibtp with invoke info
				ibtp, _ = ex.srcAdapt.QueryIBTP(ibtp.ID(), true)
				ex.rollbackIBTPForDirect(ibtp)
			} else {
				ex.sendIBTPForDirect(ex.destAdapt, ex.srcAdapt, ibtp, !ex.isIBTPBelongSrc(ibtp), false)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To] = ibtp.Index
			} else {
				ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) listenIBTPFromSrcAdaptForDirect(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromSrcAdapt Stop!")
			return
		case ibtp, ok := <-ex.srcIBTPMap[servicePair]:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.srcAdaptName)
			index := ex.getCurrentIndexFromSrc(ibtp)
			if index >= ibtp.Index {
				if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END {
					ex.sendIBTPForDirect(ex.srcAdapt, ex.destAdapt, ibtp, ex.isIBTPBelongSrc(ibtp), false)
				} else {
					ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				}
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.srcAdapt, ex.destAdapt, ibtp.From, ibtp.To, ex.isIBTPBelongSrc(ibtp))
			}

			if isRollback := ex.isIBTPRollbackForDirect(ibtp); isRollback {
				ex.rollbackIBTPForDirect(ibtp)
			} else {
				ex.sendIBTPForDirect(ex.srcAdapt, ex.destAdapt, ibtp, ex.isIBTPBelongSrc(ibtp), false)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To] = ibtp.Index
			} else {
				ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) isIBTPRollbackForDirect(ibtp *pb.IBTP) bool {
	if !ex.isIBTPBelongSrc(ibtp) || ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK || ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END {
		return false
	}

	startTimeStamp, timeoutPeriod, _, err := ex.srcAdapt.(*appchain_adapter.AppchainAdapter).GetDirectTransactionMeta(ibtp.ID())
	if err != nil {
		ex.logger.Errorf("get transaction meta with %s", ibtp.ID(), "error", err.Error())
	}

	return uint64(time.Now().Unix())-startTimeStamp > timeoutPeriod
}

func (ex *Exchanger) rollbackIBTPForDirect(ibtp *pb.IBTP) {
	ex.logger.WithFields(logrus.Fields{"typ": ibtp.Type}).Infof("src chain start rollback %s", ibtp.ID())
	ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK

	// src chain rollback
	ex.sendIBTPForDirect(ex.srcAdapt, ex.srcAdapt, ibtp, ex.isIBTPBelongSrc(ibtp), true)
	ex.logger.Infof("src chain rollback %s end", ibtp.ID())

	// src chain rollback end, notify dst chain rollback
	ex.logger.Infof("notify dst chain rollback")
	ex.sendIBTPForDirect(ex.srcAdapt, ex.destAdapt, ibtp, ex.isIBTPBelongSrc(ibtp), true)
}

func (ex *Exchanger) sendIBTPForDirect(fromAdapt, toAdapt adapt.Adapt, ibtp *pb.IBTP, isReq bool, isRollback bool) {
	if err := retry.Retry(func(attempt uint) error {
		ex.logger.Infof("start sendIBTP to Adapt:%s", toAdapt.Name())
		if err := toAdapt.SendIBTP(ibtp); err != nil {
			ex.logger.Errorf("sendIBTP to adapter:%s err:%s", toAdapt.Name(), err)
			// if err occurs, try to get new ibtp and resend
			if err, ok := err.(*adapt.SendIbtpError); ok {
				if err.NeedRetry() {
					ex.logger.Errorf("send IBTP to Adapt:%s", ex.destAdaptName, "error", err.Error())
					// query to new ibtp
					ibtp = ex.queryIBTP(fromAdapt, ibtp.ID(), isReq)
					// set ibtp rollback
					if isRollback {
						ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
					}
					// check if retry timeout
					if isTimeout := ex.isIBTPRollbackForDirect(ibtp); isTimeout {
						ex.rollbackIBTPForDirect(ibtp)
						return nil
					}
					return fmt.Errorf("retry sending ibtp")
				}
			}
		}
		return nil
	}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
		ex.logger.Panic(err)
	}
}
