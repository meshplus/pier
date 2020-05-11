package executor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

// recover recovers block height and unsent receipt to bitxhub
func (e *ChannelExecutor) recover() error {
	// recover unsent receipt
	for from, idx := range e.executeMeta {
		beginIndex, ok := e.sourceReceiptMeta[from]
		if !ok {
			beginIndex = 0
		}

		err := e.handleMissingReceipt(from, beginIndex+1, idx)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"address": from,
				"error":   err.Error(),
			}).Panic("Get missing receipt from broker")
		}
	}

	return nil
}

// getExecBlockHeight reads from level db locally to get the exact
// execution block height where it went down last time
func getExecBlockHeight(storage storage.Storage) (uint64, error) {
	v, err := storage.Get(execHeightKey())
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}

		return 0, fmt.Errorf("get exec height %w", err)
	}

	return strconv.ParseUint(string(v), 10, 64)
}

// handleMissingReceipt gets missing receipts [begin, end] from bitxhub.
// each receipt is queried by an unique id `from+e.id+index`
func (e *ChannelExecutor) handleMissingReceipt(from string, begin uint64, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing receipt is required >= 1")
	}

	entry := logger.WithFields(logrus.Fields{
		"from": from,
	})

	for ; begin <= end; begin++ {
		err := retry.Retry(func(attempt uint) error {
			entry = entry.WithFields(logrus.Fields{
				"index": begin,
			})

			// todo: query original interchain tx from lite?
			//receipt, err := e.getReceipt(from, begin)
			//if err != nil {
			//	entry.WithField("error", err).Error("Get missing execution receipt")
			//	return err
			//}

			//// send receipt back to bitxhub
			//result, err := e.agent.SendIBTP(receipt)
			//if err != nil {
			//	entry.WithField("error", err).Error("Send execution receipt to bitxhub")
			//	return err
			//}
			//
			//if !result.IsSuccess() {
			//	entry.WithField("error", err).Error("Send execution receipt to bitxhub")
			//}
			//
			//e.sourceReceiptMeta[from]++
			//
			//entry.WithFields(logrus.Fields{
			//	"status": result.Status,
			//	"hash":   result.TxHash,
			//}).Info("Get missing execution receipt from appchain")

			return nil
		}, strategy.Wait(1*time.Second))

		if err != nil {
			logger.Error(err)
			return err
		}

	}
	return nil
}

// getReceipt only generates one receipt given source chain id and interchain tx index
//func (e *ChannelExecutor) getReceipt(from string, idx uint64) (*pb.IBTP, error) {
//	ret, err := e.client.GetInMessage(from, idx)
//	if err != nil {
//		return nil, fmt.Errorf("get execution receipt message from appchain: %w", err)
//	}
//
//	ibtp, err := e.agent.GetIBTPByID(fmt.Sprintf("%s-%s-%d", from, e.id, idx))
//	if err != nil {
//		return nil, fmt.Errorf("get execution receipt for id %s-%s-%d from bitxhub:%w", from, e.id, idx, err)
//	}
//
//	return e.generateCallback(ibtp, ret)
//}
