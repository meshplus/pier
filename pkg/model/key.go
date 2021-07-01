package model

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meshplus/bitxhub-model/pb"
)

func WrapperKey(height uint64) []byte {
	return []byte(fmt.Sprintf("wrapper-%d", height))
}

func IBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("ibtp-%s", id))
}

// WrappedIBTP add IsValid field indicating if this ibtp is valid in bitxhub
type WrappedIBTP struct {
	Ibtp    *pb.IBTP
	IsValid bool
}

type LockEvent struct {
	ReceiptData []byte
	Proof       []byte
}

type BurnEvent struct {
	Token      string
	From       string
	Receipt    string
	Amount     uint64
	TxId       string
	RelayIndex uint64
	MultiSigns [][]byte
}

type UpdatedMeta []*types.Header
