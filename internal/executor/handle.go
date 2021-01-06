package executor

import (
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

// ExecuteIBTP handles ibtps of any type, which can be categorized into:
// 1. ibtp for interchain operations, then a ibtp-encoded receipt will be returned.
// And this receipt should be sent back to counterparty.
// 2. ibtp for confirmation from counterparty. This kind of ibtp is used to confirm
// the status of ibtp invoked by this pier, and it will return nothing.
func (e *ChannelExecutor) ExecuteIBTP(ibtp *pb.IBTP) (*pb.IBTP, error) {
	if ibtp == nil {
		logger.Error("empty ibtp structure")
		return nil, fmt.Errorf("nil ibtp structure")
	}

	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"from":  ibtp.From,
		"id":    ibtp.ID(),
	}).Info("Apply tx")

	switch ibtp.Type {
	case pb.IBTP_INTERCHAIN, pb.IBTP_ASSET_EXCHANGE_INIT,
		pb.IBTP_ASSET_EXCHANGE_REDEEM, pb.IBTP_ASSET_EXCHANGE_REFUND:
		return e.applyInterchainIBTP(ibtp)
	case pb.IBTP_RECEIPT_SUCCESS, pb.IBTP_RECEIPT_FAILURE, pb.IBTP_ASSET_EXCHANGE_RECEIPT:
		e.applyReceiptIBTP(ibtp)
		return nil, nil
	default:
		return nil, fmt.Errorf("wrong ibtp type")
	}
}

// applyInterchainIBTP is the handler for interchain tx. every interchain tx will have
// one receipt-type ibtp sent back to where it comes from.
// if this interchain tx has callback function, get results from the execution
// of it and set these results as callback function's args
func (e *ChannelExecutor) applyInterchainIBTP(ibtp *pb.IBTP) (*pb.IBTP, error) {
	entry := logger.WithFields(logrus.Fields{
		"from":  ibtp.From,
		"type":  ibtp.Type,
		"index": ibtp.Index,
	})

	// todo: deal with plugin returned error
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

	return response.Result, nil
}

func (e *ChannelExecutor) applyReceiptIBTP(ibtp *pb.IBTP) error {
	pd := &pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		return fmt.Errorf("unmarshal receipt type ibtp payload: %w", err)
	}

	ct := &pb.Content{}
	contentByte := pd.Content

	var err error
	if pd.Encrypted {
		contentByte, err = e.cryptor.Decrypt(contentByte, ibtp.To)
		return fmt.Errorf("decrypt ibtp payload content: %w", err)
	}

	if err := ct.Unmarshal(contentByte); err != nil {
		return fmt.Errorf("unmarshal payload content: %w", err)
	}

	// if this receipt is for executing callback function
	if ct.Func != "" {
		// if this is a callback ibtp, retry it until it worked
		// because it might be rollback in asset
		if err := retry.Retry(func(attempt uint) error {
			if err := e.execCallback(ibtp); err != nil {
				logger.Errorf("Execute callback tx: %s, retry sending tx", err.Error())
				return fmt.Errorf("execute callback tx: %w", err)
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Errorf("Execution of callback function failed: %s", err.Error())
		}
	}
	return nil
}

// execCallback is the handler for callback function of one interchain tx
func (e *ChannelExecutor) execCallback(ibtp *pb.IBTP) error {
	ibtp.From, ibtp.To = ibtp.To, ibtp.From

	// no need to send receipt for callback
	resp, err := e.client.SubmitIBTP(ibtp)
	if err != nil {
		return fmt.Errorf("handle ibtp of callback %w", err)
	}

	logger.WithFields(logrus.Fields{
		"index":  ibtp.Index,
		"type":   ibtp.Type,
		"status": resp.Status,
		"msg":    resp.Message,
	}).Info("Execute callback")

	return nil
}
