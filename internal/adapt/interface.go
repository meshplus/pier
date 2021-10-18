package adapt

import "github.com/meshplus/bitxhub-model/pb"

//go:generate mockgen -destination mock_adapt/mock_adapt.go -package mock_adapt -source interface.go
type Adapt interface {
	// Start starts adapt
	Start() error
	// Stop stops adapt
	Stop() error
	// Name get adapt name
	Name() string
	// ID   get adapt ID
	ID() string

	// MonitorIBTP listen on ibtp from dest chain
	MonitorIBTP() chan *pb.IBTP

	// QueryIBTP query ibtp by id and type, contain multi_sign
	QueryIBTP(id string, isReq bool) (*pb.IBTP, error)

	// SendIBTP check and send ibtp to dest chain
	SendIBTP(ibtp *pb.IBTP) error

	// GetServiceIDList relay/direct: AppChainAdapt, union:BxhAdapt
	GetServiceIDList() ([]string, error)

	// QueryInterchain  queryInterchain from dest chain
	QueryInterchain(serviceID string) (*pb.Interchain, error)

	// MonitorUpdatedMeta monitor validators change or block header change from AppChain on relay mode
	// 中继/直连模式监听appchain，bxh，union模式监听bxhAdapt
	MonitorUpdatedMeta() chan *[]byte

	// SendUpdatedMeta send validators change or block header change to bitXHub on relay mode
	// 中继模式发送给appchain，bxh，直连模式发送给DirectAdapt，appchain，union模式发送给unionAdapt
	SendUpdatedMeta(byte []byte) error
}
