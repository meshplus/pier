package executor

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

// applyIBTP handle ibtps of any type
func (e *ChannelExecutor) HandleIBTP(ibtp *pb.IBTP) *pb.IBTP {
	if ibtp == nil {
		logger.Error("empty ibtp structure")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"from":  ibtp.From,
		"id":    ibtp.ID(),
	}).Info("Apply tx")

	switch ibtp.Type {
	case pb.IBTP_INTERCHAIN, pb.IBTP_ASSET_EXCHANGE_INIT, pb.IBTP_ASSET_EXCHANGE_REDEEM, pb.IBTP_ASSET_EXCHANGE_REFUND, pb.IBTP_ROLLBACK:
		return e.applyInterchainIBTP(ibtp)
	case pb.IBTP_RECEIPT_SUCCESS, pb.IBTP_RECEIPT_FAILURE, pb.IBTP_ASSET_EXCHANGE_RECEIPT:
		e.applyReceiptIBTP(ibtp)
	default:
		logger.Error("wrong ibtp type")
	}

	return nil
}

// applyInterchainIBTP is the handler for interchain tx. every interchain tx will have
// one receipt-type ibtp sent back to where it comes from.
// if this interchain tx has callback function, get results from the execution
// of it and set these results as callback function's args
func (e *ChannelExecutor) applyInterchainIBTP(ibtp *pb.IBTP) *pb.IBTP {
	entry := logger.WithFields(logrus.Fields{
		"from":  ibtp.From,
		"type":  ibtp.Type,
		"index": ibtp.Index,
	})

	if e.executeMeta[ibtp.From] >= ibtp.Index {
		entry.Warn("Ignore tx")
		return nil
	}

	if e.executeMeta[ibtp.From]+1 < ibtp.Index {
		entry.WithFields(logrus.Fields{
			"required": e.executeMeta[ibtp.From] + 1,
		}).Panic("Wrong ibtp index")
	}

	// execute interchain tx, and if execution failed, try to rollback
	response, err := e.client.SubmitIBTP(ibtp)
	if err != nil {
		entry.WithField("error", err).Panic("Submit ibtp")
	}

	if response == nil || response.Result == nil {
		//entry.WithField("error", err).Panic("empty response")
	}

	if !response.Status {
		pd := &pb.Payload{}
		if err := pd.Unmarshal(ibtp.Payload); err != nil {
			entry.Panic("Unmarshal payload")
		}

		entry.WithFields(logrus.Fields{
			"result":  response.Message,
			"payload": pd,
		}).Warn("Get wrong response")
	}

	e.executeMeta[ibtp.From]++
	return response.Result
}

func (e *ChannelExecutor) applyReceiptIBTP(ibtp *pb.IBTP) {
	pd := &pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		logger.WithField("error", err).Panic("Unmarshal receipt type ibtp payload")
	}

	ct := &pb.Content{}
	contentByte := pd.Content

	var err error
	if pd.Encrypted {
		contentByte, err = e.cryptor.Decrypt(contentByte, ibtp.To)
		logger.WithField("error", err).Panic("Decrypt the content")
	}

	if err := ct.Unmarshal(contentByte); err != nil {
		logger.WithField("error", err).Panic("Unmarshal receipt type ibtp payload content")
	}

	// if this receipt is for executing callback function
	if ct.Func != "" {
		// if this is a callback ibtp, retry it until it worked
		// because it might be rollback in asset
		if err := retry.Retry(func(attempt uint) error {
			err := e.execCallback(ibtp)
			if err != nil {
				logger.Errorf("Execute callback tx: %s, retry sending tx", err.Error())
				return fmt.Errorf("execute callback tx: %w", err)
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Errorf("Execution of callback function failed: %s", err.Error())
		}
	}
}

func (e *ChannelExecutor) Rollback(ibtp *pb.IBTP, isSrcChain bool) {
	if err := retry.Retry(func(attempt uint) error {
		err := e.execRollback(ibtp, isSrcChain)
		if err != nil {
			logger.Errorf("Execute callback tx: %s, retry sending tx", err.Error())
			return fmt.Errorf("execute callback tx: %w", err)
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		logger.Errorf("Execution of callback function failed: %s", err.Error())
	}
}

// execCallback is the handler for callback function of one interchain tx
func (e *ChannelExecutor) execCallback(ibtp *pb.IBTP) error {
	ibtp.From, ibtp.To = ibtp.To, ibtp.From
	if e.callbackMeta[ibtp.From] >= ibtp.Index {
		logger.WithFields(logrus.Fields{
			"ID":           ibtp.ID(),
			"type":         ibtp.Type,
			"callbackMeta": e.callbackMeta,
		}).Info("Ignore callback")
		return nil
	}

	// no need to send receipt for callback
	resp, err := e.client.SubmitIBTP(ibtp)
	if err != nil {
		return fmt.Errorf("handle ibtp of callback %w", err)
	}

	e.callbackMeta[ibtp.From] = ibtp.Index
	logger.WithFields(logrus.Fields{
		"index":  ibtp.Index,
		"type":   ibtp.Type,
		"status": resp.Status,
		"msg":    resp.Message,
	}).Info("Execute callback")

	return nil
}

// execRollback is the handler for rollback function of one interchain tx
func (e *ChannelExecutor) execRollback(ibtp *pb.IBTP, isSrcChain bool) error {
	if isSrcChain {
		if e.callbackMeta[ibtp.From] >= ibtp.Index {
			logger.WithFields(logrus.Fields{
				"id":             ibtp.ID(),
				"callback Index": e.callbackMeta[ibtp.From],
			}).Warn("Ignore src chain rollback")
			return nil
		}
	} else {
		if e.executeMeta[ibtp.From] > ibtp.Index {
			logger.WithFields(logrus.Fields{
				"id":            ibtp.ID(),
				"execute Index": e.executeMeta[ibtp.From],
			}).Warn("Ignore dst chain rollback")
			return nil
		}
	}

	// no need to send receipt for callback
	resp, err := e.client.RollbackIBTP(ibtp, isSrcChain)
	if err != nil {
		return fmt.Errorf("rollback ibtp on source appchain %w", err)
	}

	if isSrcChain {
		e.callbackMeta[ibtp.From] = ibtp.Index
	}

	logger.WithFields(logrus.Fields{
		"index":  ibtp.Index,
		"type":   ibtp.Type,
		"status": resp.Status,
		"msg":    resp.Message,
	}).Info("Executed rollbcak")

	return nil
}
