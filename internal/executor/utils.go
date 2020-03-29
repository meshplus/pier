package executor

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

// GetIBTPByID recover missing ibtp from local storage.
// If not found, query bitxhub.
func (e *ChannelExecutor) GetIBTPByID(id string) (*pb.IBTP, error) {
	ibtp := &pb.IBTP{}
	value, err := e.storage.Get([]byte(id))
	if err != nil {
		if err == leveldb.ErrNotFound {
			// todo: how to sync all previous blocks wrappers
			// if former ibtp is not in datastore, sync all previous blocks wrappers
			ibtp := &pb.IBTP{}
			if err := retry.Retry(func(attempt uint) error {
				ibtp, err = e.agent.GetIBTPByID(id)
				if err != nil {
					logger.Errorf("can't get ibtp from bitxhub: %s, retry", err.Error())
					return err
				}
				return nil
			}, strategy.Wait(2*time.Second)); err != nil {
				logger.Panicf("Can't get ibtp from bitxhub: %s", err.Error())
			}

			return ibtp, nil
		}
		return nil, err
	}
	if err := ibtp.Unmarshal(value); err != nil {
		return nil, err
	}
	return ibtp, nil
}

// generateCallback wraps an ibtp for a receipt. It need the information
// from the corresponding incoming interchain ibtp
// if ret is "", it means this receipt is only for confirmation
// otherwise it is a callback receipt
func (e *ChannelExecutor) generateCallback(toExecute *pb.IBTP, args [][]byte) (result *pb.IBTP, err error) {
	if toExecute == nil {
		return nil, fmt.Errorf("got nil ibtp to generate receipt: %w", err)
	}
	pd := &pb.Payload{}
	if err := pd.Unmarshal(toExecute.Payload); err != nil {
		return nil, fmt.Errorf("ibtp payload unmarshal: %w", err)
	}

	as := make([][]byte, 0)
	as = append(as, pd.Args[0])
	as = append(as, args...)
	newPayload := &pb.Payload{
		SrcContractId: pd.SrcContractId,
		DstContractId: pd.DstContractId,
		Func:          pd.Callback,
		Args:          as,
	}
	pdb, err := newPayload.Marshal()
	if err != nil {
		return nil, err
	}

	return &pb.IBTP{
		From:      toExecute.From,
		To:        toExecute.To,
		Index:     toExecute.Index,
		Type:      pb.IBTP_RECEIPT,
		Timestamp: time.Now().UnixNano(),
		Proof:     nil,
		Payload:   pdb,
		Version:   toExecute.Version,
	}, nil
}

// updateHeight atomically update current working height
func (e *ChannelExecutor) updateHeight() {
	atomic.AddUint64(&e.height, 1)

	if err := e.storage.Put(execHeightKey(), []byte(strconv.FormatUint(e.height, 10))); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Put exec height into data store")
	}
}

func execHeightKey() []byte {
	return []byte("exec-height")
}
