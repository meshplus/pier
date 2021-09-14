package exchanger

import (
	"fmt"

	"github.com/meshplus/pier/internal/adapt"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) handleMissingIBTPByServicePair(begin, end uint64, srcAdapt, targetAdapt adapt.Adapt, srcService, targetService string, isReq bool) {
	for ; begin <= end; begin++ {
		ex.logger.WithFields(logrus.Fields{
			"service pair": fmt.Sprintf("%s-%s", srcService, targetService),
			"index":        begin,
		}).Info("handle missing event from:" + srcAdapt.Name())

		ibtp := srcAdapt.QueryIBTP(fmt.Sprintf("%s-%s-%d", srcService, targetService, begin), isReq)
		// todo promise success
		targetAdapt.SendIBTP(ibtp)
	}
}

func (ex *Exchanger) recover() {
	// handle src -> dest
	for _, interchain := range ex.srcServiceMeta {
		// src作为来源链的请求，req
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
		// src作为目的链的回执，resp
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
		// ①应用链作为目的链未收到的请求
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

		// ②应用链作为来源链未收到的回执；
		// ③应用链作为来源链未收到的rollback
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
