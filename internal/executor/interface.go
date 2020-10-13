package executor

import (
	"sync"

	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_executor/mock_executor.go -package mock_executor -source interface.go
type Executor interface {
	// Start starts the service of executor
	Start() error

	// Stop stops the service of executor
	Stop() error

	// HandleIBTP handles interchain ibtps from other appchains
	// and return the receipt ibtp for ack or callback
	HandleIBTP(ibtp *pb.IBTP) *pb.IBTP

	// QueryLatestMeta queries latest index map of ibtps executed on appchain
	QueryLatestMeta() *sync.Map

	// QueryLatestCallbackMeta queries latest callback index map of ibtps executed on appchain
	QueryLatestCallbackMeta() *sync.Map

	// QueryReceipt query receipt for original interchain ibtp
	QueryReceipt(from string, idx uint64, originalIBTP *pb.IBTP) (*pb.IBTP, error)
}
