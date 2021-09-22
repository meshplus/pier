package syncer

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/pkg/model"
)

type IBTPHandler func(ibtp *pb.IBTP)

type AppchainHandler func() error

type RecoverUnionHandler func(ibtp *pb.IBTP) (*rpcx.Interchain, error)

type RollbackHandler func(ibtp *pb.IBTP, ibtpId string)

var (
	ErrIBTPNotFound  = fmt.Errorf("receipt from bitxhub failed")
	ErrMetaOutOfDate = fmt.Errorf("interchain meta is out of date")
)

//go:generate mockgen -destination mock_syncer/mock_syncer.go -package mock_syncer -source syncer.go
type Syncer interface {
	// Start starts the service of syncer
	Start() error

	// Stop stops the service of syncer
	Stop() error

	// QueryInterchainMeta queries meta including interchain and receipt related meta from bitxhub
	QueryInterchainMeta(serviceID string) *pb.Interchain

	// QueryIBTP query ibtp from bitxhub by its id.
	// if error occurs, it means this ibtp is not existed on bitxhub
	QueryIBTP(ibtpID string, isReq bool) (*pb.IBTP, bool, error)

	// ListenIBTP listen on the ibtps destined for this pier from bitxhub
	ListenIBTP() <-chan *model.WrappedIBTP

	// SendIBTP sends interchain or receipt type of ibtp to bitxhub
	// if error occurs, user need to reconstruct this ibtp cause it means ibtp is invalid on bitxhub
	SendIBTP(ibtp *pb.IBTP) error

	SendIBTPWithRetry(ibtp *pb.IBTP)

	GetAssetExchangeSigns(id string) ([]byte, error)

	//getIBTPSigns gets ibtp signs from bitxhub cluster
	GetIBTPSigns(ibtp *pb.IBTP) ([]byte, error)

	//GetAppchains gets appchains from bitxhub node
	GetBitXHubIDs() ([]string, error)

	//GetInterchainById gets interchain meta by service id
	GetServiceIDs() ([]string, error)

	GetChainID() (uint64, error)

	RegisterRollbackHandler(handler RollbackHandler) error

	GetTxStatus(id string) (pb.TransactionStatus, error)
}
