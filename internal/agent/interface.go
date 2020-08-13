package agent

import (
	"context"

	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

//go:generate mockgen -destination mock_agent/mock_agent.go -package mock_agent -source interface.go
type Agent interface {
	// Appchain will query if this appchain has registered to bitxhub.
	// If this appchain related to the pier has registered, bitxhub will return
	// some basic info about this chain. Otherwise, pier will get not exist appchain error.
	Appchain() (*rpcx.Appchain, error)

	// syncBlockHeaer tries to get an channel of Headers from bitxhub.
	// Note: only the header beyond the height of sync-header request moment will be sent to this channel
	SyncBlockHeader(ctx context.Context, ch chan *pb.BlockHeader) error

	// GetBlockHeader tries to get headers whose height is in the interval of [begin, end]
	// All these headers will be sent the channel.
	GetBlockHeader(ctx context.Context, begin, end uint64, ch chan *pb.BlockHeader) error

	// SyncInterchainTxWrapper tries to get an channel of interchain tx wrapper from bitxhub.
	// Note: only the interchain tx wrappers beyond the height of sync-header request moment will be sent to this channel
	SyncInterchainTxWrappers(ctx context.Context, ch chan *pb.InterchainTxWrappers) error

	// SyncUnionInterchainTxWrapper tries to get an channel of union interchain tx wrapper from bitxhub.
	// Note: only the union interchain tx wrappers beyond the height of sync-header request moment will be sent to this channel
	SyncUnionInterchainTxWrappers(ctx context.Context, txCh chan *pb.InterchainTxWrappers) error

	// GetInterchainTxWrapper tries to get txWrappers whose height is in the interval of [begin, end]
	// All these wrappers will be sent the channel.
	GetInterchainTxWrappers(ctx context.Context, begin, end uint64, ch chan *pb.InterchainTxWrappers) error

	// SendTransaction sends the wrapped interchain tx to bitxhub
	SendTransaction(tx *pb.Transaction) (*pb.Receipt, error)

	// SendIBTP sends wrapped ibtp to bitxhub internal VM to execute
	SendIBTP(ibtp *pb.IBTP) (*pb.Receipt, error)

	// GetIBTPByID queries interchain ibtp package record
	// given an unique id of ibtp from bitxhub
	GetIBTPByID(id string) (*pb.IBTP, error)

	// GetChainMeta gets chain meta of relay chain
	GetChainMeta() (*pb.ChainMeta, error)

	GetInterchainMeta() (*rpcx.Interchain, error)

	GetAssetExchangeSigns(id string) ([]byte, error)
}
