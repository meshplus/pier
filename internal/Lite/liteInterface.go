package Lite

import "github.com/meshplus/bitxhub-model/pb"

// Lite represent abstract lite client for any blockchain
type Lite interface {
	// Start starts the service of Lite
	Start() error

	// Stop stops the service of Lite
	Stop() error

	// RecvIBTP receives valid ibtp packages
	RecvIBTP() chan *pb.IBTP

	// SendIBTP send ibtp package to appchain
	SendIBTP(ibtp *pb.IBTP) *pb.Receipt
}
