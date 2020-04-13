package executor

import (
	"fmt"
	"sort"
	"sync"

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
	var (
		validIBTPs = make([]*pb.IBTP, 0)
		ibtpM      = make(map[int]*pb.IBTP)
		wg         sync.WaitGroup
		lock       sync.Mutex
		indices    []int
	)

	wg.Add(len(wrapper.Transactions))

	for i, tx := range wrapper.Transactions {
		go func(tx *pb.Transaction, idx int) {
			defer wg.Done()

			// check if this is interchain tx
			txData := tx.GetData()
			if txData.Type != pb.TransactionData_INVOKE {
				logger.Warning("Receive a non-crosschain tx")
				return
			}

			// check if interchain ibtp is valid
			ibtp, err := tx.GetIBTP()
			if err != nil {
				panic(err)
			}
			if ibtp.Type == pb.IBTP_INTERCHAIN {
				if ibtp.To != e.id {
					return
				}
			} else if ibtp.Type == pb.IBTP_RECEIPT {
				// this receipt is not for this executor
				if ibtp.From != e.id {
					return
				}
			}

			lock.Lock()
			defer lock.Unlock()
			ibtpM[idx] = ibtp
			indices = append(indices, idx)
		}(tx, i)
	}

	wg.Wait()

	sort.Ints(indices)
	for _, idx := range indices {
		validIBTPs = append(validIBTPs, ibtpM[idx])
	}
	return validIBTPs, nil
}
