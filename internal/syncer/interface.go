package syncer

import (
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type IBTPHandler func(ibtp *pb.IBTP)

type AppchainHandler func() error

type RecoverUnionHandler func(ibtp *pb.IBTP) (*rpcx.Interchain, error)

//go:generate mockgen -destination mock_syncer/mock_syncer.go -package mock_syncer -source interface.go
type Syncer interface {
	// Start starts the service of syncer
	Start() error

	// Stop stops the service of syncer
	Stop() error

	// QueryInterchainMeta queries meta including interchain and receipt related meta from bitxhub
	QueryInterchainMeta() map[string]uint64

	// QueryIBTP query ibtp from bitxhub by its id.
	// if error occurs, it means this ibtp is not existed on bitxhub
	QueryIBTP(ibtpID string) (*pb.IBTP, error)

	// ListenIBTP listen on the ibtps destined for this pier from bitxhub
	ListenIBTP() <-chan *pb.IBTP

	// SendIBTP sends interchain or receipt type of ibtp to bitxhub
	// if error occurs, user need to reconstruct this ibtp cause it means ibtp is invalid on bitxhub
	SendIBTP(ibtp *pb.IBTP) error
}
