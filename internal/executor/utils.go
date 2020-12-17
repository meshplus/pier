package executor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
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

	typ := pb.IBTP_RECEIPT_SUCCESS
	if toExecute.Type == pb.IBTP_INTERCHAIN {
		if ct.Func == "interchainCharge" {
			res := string(args[0])
			base := 10
			if strings.HasPrefix(res, "0x") {
				res = strings.TrimPrefix(res, "0x")
				base = 16
			}
			status, err := strconv.ParseUint(res, base, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing to uint: %w", err)
			}

			if status == 0 {
				typ = pb.IBTP_RECEIPT_FAILURE
			}
		}
	} else {
		typ = pb.IBTP_ASSET_EXCHANGE_RECEIPT
	}

	return &pb.IBTP{
		From:      toExecute.From,
		To:        toExecute.To,
		Index:     toExecute.Index,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
		Proof:     nil,
		Payload:   pdb,
		Version:   toExecute.Version,
	}, nil
}
