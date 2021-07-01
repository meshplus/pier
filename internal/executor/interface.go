package executor

import (
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
)

//go:generate mockgen -destination mock_executor/mock_executor.go -package mock_executor -source interface.go
type Executor interface {
	// Start starts the service of executor
	Start() error

	// Stop stops the service of executor
	Stop() error

	// ExecuteIBTP handles interchain ibtps from other appchains
	// and return the receipt ibtp for ack or callback
	ExecuteIBTP(wIbtp *model.WrappedIBTP) (*pb.IBTP, error)

	// Rollback rollbacks ibtp on appchain
	Rollback(ibtp *pb.IBTP, isSrcChain bool)

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

	// SendBurnEvent send unlock event to plugins
	SendBurnEvent(unLockEvt *pb.UnLock) error

	// QueryFilterLockStart query the lasted lock event height in appchain
	QueryFilterLockStart(appchainIndex int64) int64

	// QueryLockEventByIndex query A lock event by lock index
	QueryLockEventByIndex(index int64) *pb.LockEvent

	// QueryAppchainIndex query the lasted lock event index from appchain
	QueryAppchainIndex() int64

	// QueryRelayIndex query the lasted burn event index from appchain
	QueryRelayIndex() int64
}
