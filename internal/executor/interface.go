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
	ExecuteIBTP(ibtp *pb.IBTP) *pb.IBTP

	// QueryMeta queries latest index map of ibtps executed on appchain
	// For the returned map, key is the source chain ID,
	// and value is the latest index of tx executed on appchain
	QueryMeta() map[string]uint64

	// QueryIBTPReceipt query receipt for original interchain ibtp
	QueryIBTPReceipt(from string, index uint64) *pb.IBTP
}
