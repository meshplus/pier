package monitor

import "github.com/meshplus/bitxhub-model/pb"

//go:generate mockgen -destination mock_monitor/mock_monitor.go -package mock_monitor -source interface.go
type Monitor interface {
	// Start starts the service of monitor
	Start() error

	// Stop stops the service of monitor
	Stop() error

	// listen on interchain ibtp from now on
	ListenOnIBTP() chan *pb.IBTP

	// query historical ibtp by its id
	QueryIBTP(id string) (*pb.IBTP, error)

	// QueryLatestMeta queries latest index map of ibtps executed on appchain
	QueryLatestMeta() map[string]uint64
}
