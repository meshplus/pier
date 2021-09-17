package adapt

import "github.com/meshplus/bitxhub-model/pb"

//go:generate mockgen -destination mock_adapt/mock_adapt.go -package mock_adapt -source interface.go
type Adapt interface {
	// Start starts adapt
	Start() error
	// Stop stops adapt
	Stop() error

	// MonitorIBTP listen on ibtp from dest chain
	MonitorIBTP() chan *pb.IBTP

	// QueryIBTP query ibtp by id and type, contain mutilsign
	QueryIBTP(id string, isReq bool) (*pb.IBTP, error)

	// SendIBTP check and send ibtp to dest chain
	SendIBTP(ibtp *pb.IBTP) error

	// GetServiceIDList getServiceIDList from dest chain
	GetServiceIDList() ([]string, error)

	// QueryInterchain  queryInterchain from dest chain
	QueryInterchain(serviceID string) (*pb.Interchain, error)
}
