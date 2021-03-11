package executor

import (
	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_executor/mock_executor.go -package mock_executor -source interface.go
type Executor interface {
	// Start starts the service of executor
	Start() error

	// Stop stops the service of executor
	Stop() error

	// ExecuteIBTP handles interchain ibtps from other appchains
	// and return the receipt ibtp for ack or callback
	ExecuteIBTP(ibtp *pb.IBTP) (*pb.IBTP, error)

	// QueryInterchainMeta queries latest index map of ibtps executed on appchain
	// For the returned map, key is the source chain ID,
	// and value is the latest index of tx executed on appchain
	QueryInterchainMeta() map[string]uint64

	// QueryCallbackMeta queries latest index map of ibtps callbacks executed on appchain
	// For the returned map, key is the destination chain ID,
	// and value is the latest index of callback executed on appchain
	QueryCallbackMeta() map[string]uint64

	// QueryIBTPReceipt query receipt for original interchain ibtp
	QueryIBTPReceipt(originalIBTP *pb.IBTP) (*pb.IBTP, error)
}
