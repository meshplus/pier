package executor

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

// ExecuteIBTP handles ibtps of any type, which can be categorized into:
// 1. ibtp for interchain operations, then a ibtp-encoded receipt will be returned.
// And this receipt should be sent back to counterparty.
// 2. ibtp for confirmation from counterparty. This kind of ibtp is used to confirm
// the status of ibtp invoked by this pier, and it will return nothing.
func (e *ChannelExecutor) ExecuteIBTP(wIbtp *model.WrappedIBTP) (*pb.IBTP, error) {
	if wIbtp == nil || wIbtp.Ibtp == nil {
		e.logger.Error("empty ibtp structure")
		return nil, fmt.Errorf("nil ibtp structure")
	}
	ibtp := wIbtp.Ibtp
	e.logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"from":  ibtp.From,
		"to":    ibtp.To,
	}).Info("Apply tx")

	switch ibtp.Type {
	// todo: Temporary annotation
	//case pb.IBTP_INTERCHAIN, pb.IBTP_ROLLBACK:
	//	return e.applyInterchainIBTP(wIbtp)
	case pb.IBTP_RECEIPT_SUCCESS, pb.IBTP_RECEIPT_FAILURE:
		err := e.applyReceiptIBTP(wIbtp)
		return nil, err
	default:
		return nil, fmt.Errorf("wrong ibtp type")
	}
}

// applyInterchainIBTP is the handler for interchain tx. every interchain tx will have
// one receipt-type ibtp sent back to where it comes from.
// if this interchain tx has callback function, get results from the execution
// of it and set these results as callback function's args
func (e *ChannelExecutor) applyInterchainIBTP(wIbtp *model.WrappedIBTP) (*pb.IBTP, error) {
	// todo: Temporary annotation
	//ibtp := wIbtp.Ibtp
	//entry := e.logger.WithFields(logrus.Fields{
	//	"from":  ibtp.From,
	//	"to":    ibtp.To,
	//	"type":  ibtp.Type,
	//	"index": ibtp.Index,
	//})
	//
	////todo: deal with plugin returned error
	////execute interchain tx, and if execution failed, try to rollback
	//response, err := e.client.SubmitIBTP(ibtp)
	//if err != nil {
	//	entry.WithField("error", err).Panic("Submit ibtp")
	//}
	//
	//if response == nil || response.Result == nil {
	//	entry.WithField("error", err).Panic("empty response")
	//}
	//
	//if !response.Status {
	//	pd := &pb.Payload{}
	//	if err := pd.Unmarshal(response.Result.Payload); err != nil {
	//		entry.Panic("Unmarshal payload")
	//	}
	//
	//	entry.WithFields(logrus.Fields{
	//		"result":  response.Message,
	//		"payload": pd,
	//	}).Warn("Get wrong response, need rollback on source chain")
	//}
	//
	//return response.Result, nil
	return nil, nil
}

func (e *ChannelExecutor) applyReceiptIBTP(wIbtp *model.WrappedIBTP) error {
	// if this ibtp receipt fail, no need to rollback
	if !wIbtp.IsValid {
		return nil
	}

	ibtp := wIbtp.Ibtp
	pd := &pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		return fmt.Errorf("unmarshal receipt type ibtp payload: %w", err)
	}

	if pd.Encrypted {
		contentByte, err := e.cryptor.Decrypt(pd.Content, ibtp.To)
		if err != nil {
			return fmt.Errorf("decrypt ibtp payload content: %w", err)
		}

		pd.Content = contentByte
		pd.Encrypted = false
		payload, err := pd.Marshal()
		if err != nil {
			return err
		}

		ibtp.Payload = payload
	}

	// todo: Temporary annotation
	//if _, err := e.client.SubmitIBTP(ibtp); err != nil {
	//	e.logger.Errorf("Execute callback tx: %s", err.Error())
	//	return fmt.Errorf("execute callback tx: %w", err)
	//}

	return nil
}

func (e *ChannelExecutor) Rollback(ibtp *pb.IBTP, isSrcChain bool) {
}
