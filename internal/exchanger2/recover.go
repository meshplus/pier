package exchanger

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) handleMissingIBTPByServicePair(begin, end uint64, fromAdapt, toAdapt adapt.Adapt, srcService, targetService string, isReq bool) {
	for ; begin <= end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": fmt.Sprintf("%s-%s", srcService, targetService),
			"index":        begin,
		}).Info("handle missing event from:" + fromAdapt.Name())

		if err := retry.Retry(func(attempt uint) error {
			ibtp, err := fromAdapt.QueryIBTP(fmt.Sprintf("%s-%s-%d", srcService, targetService, begin), isReq)
			if err != nil {
				ex.logger.Errorf("queryIBTP from Adapt:%s", fromAdapt.Name(), "error", err.Error())
				return err
			}
			err = toAdapt.SendIBTP(ibtp)
			if err != nil {
				ex.logger.Errorf("send IBTP to Adapt:%s", toAdapt.Name(), "error", err.Error())
				return err
			}
			return nil
		}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
			ex.logger.Panic(err)
		}

	}
}

func (ex *Exchanger) recover() {
	// handle src -> dest
	for _, interchain := range ex.srcServiceMeta {

		for k, count := range interchain.InterchainCounter {
			destCount, ok := ex.destServiceMeta[interchain.ID].InterchainCounter[k]
			if !ok {
				panic(fmt.Sprintf("service can not found in destAdapt : %s", k))
			}
			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.srcAdapt, ex.destAdapt, interchain.ID, k, true)
			// success then equal index
			ex.destServiceMeta[interchain.ID].InterchainCounter[k] = count
		}
		for k, count := range interchain.SourceReceiptCounter {
			destCount, ok := ex.destServiceMeta[interchain.ID].SourceReceiptCounter[k]
			if !ok {
				panic(fmt.Sprintf("service can not found in destAdapt : %s", k))
			}
			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.srcAdapt, ex.destAdapt, k, interchain.ID, false)
			ex.destServiceMeta[interchain.ID].SourceReceiptCounter[k] = count
		}
	}

	// handle dest -> src
	for _, interchain := range ex.destServiceMeta {
		for k, count := range interchain.SourceInterchainCounter {
			destCount, ok := ex.srcServiceMeta[interchain.ID].SourceInterchainCounter[k]
			if !ok {
				panic(fmt.Sprintf("service can not found in adapt0 : %s", k))
			}

			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.destAdapt, ex.srcAdapt, k, interchain.ID, true)
			ex.srcServiceMeta[interchain.ID].SourceInterchainCounter[k] = count
		}

		for k, count := range interchain.ReceiptCounter {
			destCount, ok := ex.srcServiceMeta[interchain.ID].ReceiptCounter[k]
			if !ok {
				panic(fmt.Sprintf("service can not found in adapt0 : %s", k))
			}

			// handle unsentIBTP : query IBTP -> sendIBTP
			var begin = destCount + 1
			ex.handleMissingIBTPByServicePair(begin, count, ex.destAdapt, ex.srcAdapt, interchain.ID, k, false)
			ex.srcServiceMeta[interchain.ID].ReceiptCounter[k] = count
		}
	}

}
