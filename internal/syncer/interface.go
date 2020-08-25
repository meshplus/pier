package syncer

import (
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type IBTPHandler func(ibtp *pb.IBTP)

type RouterHandler func() error

type RecoverUnionHandler func(ibtp *pb.IBTP) (*rpcx.Interchain, error)

//go:generate mockgen -destination mock_syncer/mock_syncer.go -package mock_syncer -source interface.go
type Syncer interface {
	// Start starts the service of syncer
	Start() error

	// Stop stops the service of syncer
	Stop() error

	RegisterIBTPHandler(handler IBTPHandler) error

	RegisterRouterHandler(handler RouterHandler) error

	RegisterRecoverHandler(handleRecover RecoverUnionHandler) error
}
