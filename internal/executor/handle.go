package executor

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

// applyMerkleWrapper validates block wrapper from bitxhub and unwrap its ibtps
// if there are any interchain txs targeting to this pier;
// if so, it will handle these interchain txs differently according to their type
func (e *ChannelExecutor) applyMerkleWrapper(wrapper *pb.MerkleWrapper) {
	logger.WithFields(logrus.Fields{
		"height": wrapper.BlockHeader.Number,
		"count":  len(wrapper.Transactions),
	}).Info("Execute merkle wrapper")

	ibtps, err := e.verify(wrapper)
	if err != nil {
		// todo: what should exec do when wrapper is wrong
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Panic("Invalid wrapper")

		return
	}

	defer func() {
		logger.WithFields(logrus.Fields{
			"height": wrapper.BlockHeader.Number,
		}).Info("Finish merkle wrapper")

		e.updateHeight()
	}()

	for _, ibtp := range ibtps {
		if err := e.applyIBTP(ibtp); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("Execute ibtp")
		}
	}
}

// applyIBTP handle ibtps of any type
func (e *ChannelExecutor) applyIBTP(ibtp *pb.IBTP) error {
	if ibtp == nil {
		return fmt.Errorf("empty ibtp structure")
	}

	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"from":  ibtp.From,
		"id":    ibtp.ID(),
	}).Info("Apply tx")

	switch ibtp.Type {
	case pb.IBTP_INTERCHAIN:
		e.applyInterchainIBTP(ibtp)
	case pb.IBTP_RECEIPT:
		e.applyReceiptIBTP(ibtp)
	default:
		return fmt.Errorf("wrong ibtp type")
	}

	return nil
}

// applyInterchainIBTP is the handler for interchain tx. every interchain tx will have
// one receipt-type ibtp sent back to where it comes from.
// if this interchain tx has callback function, get results from the execution
// of it and set these results as callback function's args
func (e *ChannelExecutor) applyInterchainIBTP(ibtp *pb.IBTP) {
	entry := logger.WithFields(logrus.Fields{
		"from":  ibtp.From,
		"type":  ibtp.Type,
		"index": ibtp.Index,
	})

	if e.executeMeta[ibtp.From] >= ibtp.Index {
		entry.Warn("Ignore tx")
		return
	}

	if e.executeMeta[ibtp.From]+1 < ibtp.Index {
		entry.WithFields(logrus.Fields{
			"required": e.executeMeta[ibtp.From],
		}).Panic("Wrong ibtp index")
	}

	// execute interchain tx, and if execution failed, try to rollback
	response, err := e.client.SubmitIBTP(ibtp)
	if err != nil {
		entry.WithField("error", err).Panic("Submit ibtp")
	}

	if response == nil || response.Result == nil {
		entry.WithField("error", err).Panic("empty response")
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

	if err := retry.Retry(func(attempt uint) error {
		receipt, err := e.agent.SendIBTP(response.Result)
		if err != nil {
			entry.Error(err)
			return err
		}

		if !receipt.IsSuccess() {
			entry.WithField("error", string(receipt.Ret)).Error("Send receipt IBTP")
			return nil
		}

		entry.Info("Send receipt IBTP")

		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		entry.Panicf("Can't send rollback ibtp back to bitxhub: %s", err.Error())
	}

	e.executeMeta[ibtp.From]++
	e.sourceReceiptMeta[ibtp.From]++
}

func (e *ChannelExecutor) applyReceiptIBTP(ibtp *pb.IBTP) {
	pd := &pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		logger.WithField("error", err).Panic("Unmarshal receipt type ibtp payload")
	}

	// if this receipt is for executing callback function
	if pd.Func != "" {
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

// execCallback is the handler for callback function of one interchain tx
func (e *ChannelExecutor) execCallback(ibtp *pb.IBTP) error {
	ibtp.From, ibtp.To = ibtp.To, ibtp.From
	if e.callbackMeta[ibtp.From] >= ibtp.Index {
		logger.WithFields(logrus.Fields{
			"from":  ibtp.From,
			"type":  ibtp.Type,
			"index": ibtp.Index,
		}).Info("Ignore callback")
		return nil
	}

	// no need to send receipt for callback
	_, err := e.client.SubmitIBTP(ibtp)
	if err != nil {
		return fmt.Errorf("handle ibtp of callback %w", err)
	}

	e.callbackMeta[ibtp.From] = ibtp.Index
	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
	}).Info("Execute callback")

	return nil
}

// handleMissingIBTP fetches missing ibtps [begin, end) from bitxhub
func (e *ChannelExecutor) handleMissingIBTP(from string, begin uint64, end uint64) error {
	for i := begin; i < end; i++ {
		ibtp, err := e.GetIBTPByID(fmt.Sprintf("%s-%s-%d", from, e.id, i))
		if err != nil {
			return fmt.Errorf("fetch receipt for id %s-%s-%d from bitxhub: %w", from, e.id, i, err)
		}

		if err := e.applyIBTP(ibtp); err != nil {
			return err
		}
	}
	return nil
}
