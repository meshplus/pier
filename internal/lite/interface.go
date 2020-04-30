package lite

import "github.com/meshplus/bitxhub-model/pb"

//go:generate mockgen -destination mock_lite/mock_lite.go -package mock_lite -source interface.go
type Lite interface {
	// Start starts service of lite client
	Start() error

	// Stop stops service of lite client
	Stop() error

	// QueryHeader gets block header given block height
	QueryHeader(height uint64) (*pb.BlockHeader, error)
}
