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
	SyncBlockHeader(ctx context.Context) (chan *pb.BlockHeader, error)

	// GetHeader tries to get headers whose height is in the interval of [begin, end]
	// All these headers will be sent the channel.
	GetHeader(begin, end uint64) (chan *pb.BlockHeader, error)

	// SyncInterchainTxWrapper tries to get an channel of interchain tx wrapper from bitxhub.
	// Note: only the interchain tx wrappers beyond the height of sync-header request moment will be sent to this channel
	SyncInterchainTxWrapper(ctx context.Context) (chan *pb.InterchainTxWrapper, error)

	// GetInterchainTxWrapper tries to get txWrappers whose height is in the interval of [begin, end]
	// All these wrappers will be sent the channel.
	GetInterchainTxWrapper(begin, end uint64) (chan *pb.InterchainTxWrapper, error)

	// SendTransaction sends the wrapped interchain tx to bitxhub
	SendTransaction(tx *pb.Transaction) (*pb.Receipt, error)

	// SendIBTP sends wrapped ibtp to bitxhub internal VM to execute
	SendIBTP(ibtp *pb.IBTP) (*pb.Receipt, error)

	// GetIBTPByID queries interchain ibtp package record
	// given an unique id of ibtp from bitxhub
	GetIBTPByID(id string) (*pb.IBTP, error)

	// GetChainMeta gets chain meta of relay chain
	GetChainMeta() (*pb.ChainMeta, error)
}
