package bxhLite

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
)

// verifyWrapper verifies the basic of merkle wrapper from bitxhub
func (lite *BxhLite) verifyWrapper(w *pb.MerkleWrapper) (bool, error) {
	if w == nil || w.BlockHeader == nil {
		return false, fmt.Errorf("empty wrapper or block header")
	}

	// validate if the wrapper is from bitxhub
	if w.BlockHeader.Number != lite.getDemandHeight() {
		return false, fmt.Errorf("wrong height of wrapper from bitxhub")
	}

	// todo: wait for bitxhub to provide signatures for wrapper, now verifyTxs always return true
	if uint64(len(w.Signatures)) <= lite.quorum {
		return true, fmt.Errorf("%d signatures not meet the requirement of quorum %d", len(w.Signatures), lite.quorum)
	}

	count := uint64(0)
	var wg sync.WaitGroup

	for _, validator := range lite.validators {
		sign, ok := w.Signatures[validator.String()]
		if ok {
			wg.Add(1)
			go func(vlt types.Address, sign []byte) {
				if isValid, _ := asym.Verify(asym.ECDSASecp256r1, sign, w.SignHash().Bytes(), vlt); isValid {
					atomic.AddUint64(&count, 1)
				}
				wg.Done()
			}(validator, sign)
		}
	}

	wg.Wait()
	if count <= lite.quorum {
		return true, fmt.Errorf("invalid signature")
	}

	return true, nil
}

// verifyTxs verifies merkleWrapper received
func (lite *BxhLite) verifyTxs(wrapper *pb.MerkleWrapper) ([]*pb.IBTP, error) {
	if wrapper == nil {
		return nil, nil
	}

	if len(wrapper.Transactions) == 0 {
		return nil, nil
	}

	valid, err := wrapper.VerifyMerkleWrapper()
	if err != nil {
		return nil, fmt.Errorf("verifyTxs merkle wrapper: %w", err)
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
				if ibtp.To != lite.chainID {
					return
				}
			} else if ibtp.Type == pb.IBTP_RECEIPT {
				// this receipt is not for this executor
				if ibtp.From != lite.chainID {
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
