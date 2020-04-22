package monitor

import "github.com/meshplus/bitxhub-model/pb"

//go:generate mockgen -destination mock_monitor/mock_monitor.go -package mock_monitor -source interface.go
type Monitor interface {
	// Start starts the service of monitor
	Start() error

	// Stop stops the service of monitor
	Stop() error

	FetchIBTP() chan *pb.IBTP
}
