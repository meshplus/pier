package exchanger

import (
	"encoding/binary"
	"fmt"
	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/adapt/appchain_adapter"
	"github.com/sirupsen/logrus"
	"time"
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
				// check if IBTP_RECEIPT_ROLLBACK && IBTP_RECEIPT_ROLLBACK_END
				if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK {
					// dstChain receive rollback, need rollback
					if err := retry.Retry(func(attempt uint) error {
						ex.logger.Infof("receive rollback ibtp from %s, start sendIBTP to adapter: %s", ibtp.GetFrom(), ex.srcAdaptName)
						if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
							// if err occurs, try to get new ibtp and resend
							if err, ok := err.(*adapt.SendIbtpError); ok {
								if err.NeedRetry() {
									// query to new ibtp
									ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
									ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
									return fmt.Errorf("retry sending ibtp")
								}
							}
						}
						return nil
					}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
						ex.logger.Panic(err)
					}
				} else if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END {
					// receive rollback end from dstChain, srcChain change transaction status
					if err := retry.Retry(func(attempt uint) error {
						ex.logger.Infof("receive rollback end, start sendIBTP to adapter: %s", ex.srcAdaptName)
						if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
							// if err occurs, try to get new ibtp and resend
							if err, ok := err.(*adapt.SendIbtpError); ok {
								if err.NeedRetry() {
									// query to new ibtp
									ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
									return fmt.Errorf("retry sending ibtp")
								}
							}
						}
						return nil
					}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
						ex.logger.Panic(err)
					}
				} else {
					ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				}
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.destAdapt, ex.srcAdapt, ibtp.From, ibtp.To, !ex.isIBTPBelongSrc(ibtp))
			}

			var isTimeout bool
			// only check IBTP_RECEIPT_SUCCESS && IBTP_RECEIPT_FAILURE
			if ex.isIBTPBelongSrc(ibtp) && ibtp.Type != pb.IBTP_RECEIPT_ROLLBACK && ibtp.Type != pb.IBTP_RECEIPT_ROLLBACK_END {
				startTimeStamp, timeoutPeriod, err := ex.srcAdapt.(*appchain_adapter.AppchainAdapter).GetTransactionMeta(ibtp.ID())
				if err != nil {
					ex.logger.Errorf("get transaction meta with %s", ibtp.ID(), "error", err.Error())
				}
				// transaction timeout
				if uint64(time.Now().Unix())-startTimeStamp > timeoutPeriod {
					isTimeout = true
					ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
				}
			}

			if err := retry.Retry(func(attempt uint) error {
				if isTimeout {
					ex.logger.Infof("srcChain rollback, start sendIBTP to adapter: %s", ex.srcAdaptName)
				} else {
					ex.logger.Infof("start sendIBTP to adapter: %s", ex.srcAdaptName)
				}
				if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
					ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
							// if transaction timeout, src chain rollback
							if isTimeout {
								ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
							}
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			// if transaction timeout, dst chain need rollback
			if isTimeout {
				// get outMessage and notify dst chain rollback
				ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
				ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
				if err := retry.Retry(func(attempt uint) error {
					ex.logger.Infof("notify dstChain %s rollback, start sendIBTP to adapter: %s", ibtp.GetTo(), ex.destAdaptName)
					if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
						ex.logger.Errorf("send IBTP to Adapt:%s", ex.destAdaptName, "error", err.Error())
						// if err occurs, try to get new ibtp and resend
						if err, ok := err.(*adapt.SendIbtpError); ok {
							if err.NeedRetry() {
								// query to new ibtp
								ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
								ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
								return fmt.Errorf("retry sending ibtp")
							}
						}
					}
					return nil
				}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
					ex.logger.Panic(err)
				}
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
				if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END && !ex.isIBTPBelongSrc(ibtp) {
					if err := retry.Retry(func(attempt uint) error {
						ex.logger.Infof("start sendIBTP to Adapt:%s", ex.destAdaptName)
						if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
							// if err occurs, try to get new ibtp and resend
							if err, ok := err.(*adapt.SendIbtpError); ok {
								if err.NeedRetry() {
									ex.logger.Errorf("send IBTP to Adapt:%s", ex.destAdaptName, "error", err.Error())
									// query to new ibtp
									ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
									return fmt.Errorf("retry sending ibtp")
								}
							}
						}
						return nil
					}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
						ex.logger.Panic(err)
					}
				}
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.srcAdapt, ex.destAdapt, ibtp.From, ibtp.To, ex.isIBTPBelongSrc(ibtp))
			}

			var isTimeout bool
			// transaction timeout
			// ensure ibtp belong src chain
			if ex.isIBTPBelongSrc(ibtp) {
				startTimeStamp := int64(binary.BigEndian.Uint64(ibtp.GetExtra()))
				if time.Now().Unix()-startTimeStamp > ibtp.GetTimeoutHeight() {
					isTimeout = true
					// src chain rollback
					ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
					if err := retry.Retry(func(attempt uint) error {
						ex.logger.Infof("srcChain rollback, start send IBTP to Adapt:%s", ex.srcAdaptName)
						if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
							// if err occurs, try to get new ibtp and resend
							if err, ok := err.(*adapt.SendIbtpError); ok {
								if err.NeedRetry() {
									ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
									// query to new ibtp
									ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
									ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
									return fmt.Errorf("retry sending ibtp")
								}
							}
						}
						return nil
					}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
						ex.logger.Panic(err)
					}
				}
			}

			if err := retry.Retry(func(attempt uint) error {
				if isTimeout {
					ex.logger.Infof("dstChain rollback, start sendIBTP to Adapt:%s", ex.destAdaptName)
				} else {
					ex.logger.Infof("start sendIBTP to Adapt:%s", ex.destAdaptName)
				}
				if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.destAdaptName, "error", err.Error())
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
							if isTimeout {
								// dst chain rollback
								ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
							}
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To] = ibtp.Index
			} else {
				ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}
