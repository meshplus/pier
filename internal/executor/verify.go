package executor

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
)

// verify verifies merkleWrapper received
func (e *ChannelExecutor) verify(wrapper *pb.MerkleWrapper) ([]*pb.IBTP, error) {
	if wrapper == nil {
		return nil, nil
	}

	if len(wrapper.Transactions) == 0 {
		return nil, nil
	}

	valid, err := wrapper.VerifyMerkleWrapper()
	if err != nil {
		return nil, fmt.Errorf("verify merkle wrapper: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid merkle wrapper #%d", wrapper.BlockHeader.Number)
	}

	// filter those ibtps whose destination is this pier
	var ibtps = make([]*pb.IBTP, 0)

	for _, tx := range wrapper.Transactions {
		// check if this is interchain tx
		txData := tx.GetData()

		if txData.Type != pb.TransactionData_INVOKE {
			logger.Warning("Receive a non-crosschain tx")
			continue
		}

		// check if interchain ibtp is valid
		ibtp, err := tx.GetIBTP()
		if err != nil {
			panic(err)
		}
		if ibtp.Type == pb.IBTP_INTERCHAIN {
			if ibtp.To != e.id {
				continue
			}
		} else if ibtp.Type == pb.IBTP_RECEIPT {
			// this receipt is not for this executor
			if ibtp.From != e.id {
				continue
			}
		}

		ibtps = append(ibtps, ibtp)
	}
	return ibtps, nil
}
