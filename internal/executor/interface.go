package executor

import (
	"github.com/ethereum/go-ethereum/event"
	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_executor/mock_executor.go -package mock_executor -source interface.go
type Executor interface {
	// Start starts the service of executor
	Start() error

	// Stop stops the service of executor
	Stop() error

	// HandleIBTP handles interchain ibtps from other appchains
	HandleIBTP(ibtp *pb.IBTP)

	// SubscribeReceipt subscribes the execution receipt to return to other appchains
	SubscribeReceipt(chan<- *pb.IBTP) event.Subscription
}
