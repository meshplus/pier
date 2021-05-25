package syncer

import (
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/pkg/model"
)

type IBTPHandler func(ibtp *pb.IBTP)

type AppchainHandler func() error

type RecoverUnionHandler func(ibtp *pb.IBTP) (*rpcx.Interchain, error)

type RollbackHandler func(ibtp *pb.IBTP)

//go:generate mockgen -destination mock_syncer/mock_syncer.go -package mock_syncer -source interface.go
type Syncer interface {
	// Start starts the service of syncer
	Start() error

	// Stop stops the service of syncer
	Stop() error

	// QueryInterchainMeta queries meta including interchain and receipt related meta from bitxhub
	QueryInterchainMeta() *pb.Interchain

	// QueryIBTP query ibtp from bitxhub by its id.
	// if error occurs, it means this ibtp is not existed on bitxhub
	QueryIBTP(ibtpID string) (*pb.IBTP, bool, error)

	// ListenIBTP listen on the ibtps destined for this pier from bitxhub
	ListenIBTP() <-chan *model.WrappedIBTP

	// ListenUnescrow listen on the UnescrowEvent destined for this pier-related appchain from bitxhub
	ListenUnescrow(unescrowEvent *model.UnescrowEvent) error

	// SendIBTP sends interchain or receipt type of ibtp to bitxhub
	// if error occurs, user need to reconstruct this ibtp cause it means ibtp is invalid on bitxhub
	SendIBTP(ibtp *pb.IBTP) error

	// SendUpdateMeta sends to-update appchain meta(like block headers in appchain) to bitxhub
	SendUpdateMeta(meta model.UpdatedMeta) error

	// SendMintEvent sends asset exchange event(finalized) in appchain to bitxhub
	SendMintEvent(mintEvnt *model.MintEvent) error

	GetAssetExchangeSigns(id string) ([]byte, error)

	//getIBTPSigns gets ibtp signs from bitxhub cluster
	GetIBTPSigns(ibtp *pb.IBTP) ([]byte, error)

	//GetAppchains gets appchains from bitxhub node
	GetAppchains() ([]*appchainmgr.Appchain, error)

	//GetInterchainById gets interchain meta by appchain id
	GetInterchainById(from string) *pb.Interchain

	// RegisterRecoverHandler registers handler that recover ibtps from bitxhub
	RegisterRecoverHandler(RecoverUnionHandler) error

	// RegisterAppchainHandler registers handler that fetch appchains information
	RegisterAppchainHandler(handler AppchainHandler) error

	RegisterRollbackHandler(handler RollbackHandler) error
}
