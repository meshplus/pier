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

	// HandleIBTP handles interchain ibtps from other appchains
	// and return the receipt ibtp for ack or callback
	HandleIBTP(ibtp *pb.IBTP) *pb.IBTP

	// Rollback rollbacks ibtp on appchain
	Rollback(ibtp *pb.IBTP, isSrcChain bool)

	// QueryLatestMeta queries latest index map of ibtps executed on appchain
	QueryLatestMeta() map[string]uint64

	// QueryReceipt query receipt for original interchain ibtp
	QueryReceipt(from string, idx uint64, originalIBTP *pb.IBTP) (*pb.IBTP, error)

	QueryDstRollbackMeta() map[string]uint64
}
