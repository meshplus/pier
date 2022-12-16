package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) listenIBTPFromSrcAdaptForRelay(servicePair string) {
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
			if err := retry.Retry(func(attempt uint) error {
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
			}, strategy.Wait(5*time.Second)); err != nil {
				ex.logger.Panic(err)
			}
		}
	}
}
func (ex *Exchanger) listenIBTPFromDestAdaptForRelay(servicePair string) {
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
			if err := retry.Retry(func(attempt uint) error {
				if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
					ex.logger.Error(err)
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							ex.logger.Errorf("send IBTP to Adapt:%s", ex.srcAdaptName, "error", err.Error())
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Wait(5*time.Second)); err != nil {
				ex.logger.Panic(err)
			}
		}
	}
}
