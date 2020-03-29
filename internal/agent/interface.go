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

	// SyncMerkleWrapper tries to get an channel of merkleWrappers from bitxhub.
	// A merkleWrapper includes block header and interchain txs.
	// Note: only the wrapper beyond the height of sync-wrapper request moment will be sent to this channel
	SyncMerkleWrapper(ctx context.Context) (chan *pb.MerkleWrapper, error)

	// GetMerkleWrapper tries to get wrappers whose height is in the interval of [begin, end]
	// All these wrappers will be sent the channel.
	GetMerkleWrapper(begin, end uint64) (chan *pb.MerkleWrapper, error)

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
