package executor

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

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

	ct := &pb.Content{}
	contentByte := pd.Content

	if pd.Encrypted {
		contentByte, err = e.cryptor.Decrypt(contentByte, toExecute.From)
		if err != nil {
			return nil, err
		}
	}

	if err := ct.Unmarshal(contentByte); err != nil {
		return nil, fmt.Errorf("ibtp payload content unmarshal: %w", err)
	}

	as := make([][]byte, 0)
	as = append(as, ct.Args[0])
	as = append(as, args...)
	newContent := &pb.Content{
		SrcContractId: ct.SrcContractId,
		DstContractId: ct.DstContractId,
		Func:          ct.Callback,
		Args:          as,
	}
	ctb, err := newContent.Marshal()
	if err != nil {
		return nil, err
	}
	if pd.Encrypted {
		ctb, err = e.cryptor.Encrypt(ctb, toExecute.To)
		if err != nil {
			return nil, err
		}
	}
	newPayload := &pb.Payload{
		Encrypted: pd.Encrypted,
		Content:   ctb,
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
