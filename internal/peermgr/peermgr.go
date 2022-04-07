package peermgr

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
)

type MessageHandler func(network.Stream, *pb.Message)
type ConnectHandler func(string)

//go:generate mockgen -destination mock_peermgr/mock_peermgr.go -package mock_peermgr -source peermgr.go
type PeerManager interface {
	DHTManager

	// Start
	Start() error

	// Stop
	Stop() error

	// AsyncSend sends message to peer with peer info.
	AsyncSend(string, *pb.Message) error

	Connect(info *peer.AddrInfo) (string, error)

	// SendWithStream sends message using existed stream
	SendWithStream(network.Stream, *pb.Message) (*pb.Message, error)

	// AsyncSendWithStream sends message using existed stream
	AsyncSendWithStream(network.Stream, *pb.Message) error

	// Send sends message waiting response
	Send(string, *pb.Message) (*pb.Message, error)

	// Peers
	Peers() map[string]*peer.AddrInfo

	// RegisterMsgHandler
	RegisterMsgHandler(pb.Message_Type, MessageHandler) error

	// RegisterMultiMsgHandler
	RegisterMultiMsgHandler([]pb.Message_Type, MessageHandler) error

	// RegisterConnectHandler
	RegisterConnectHandler(ConnectHandler) error
}

type DHTManager interface {
	// Search for peers who are able to provide a given key
	FindProviders(id string) (string, error)

	// Provide adds the given cid to the content routing system. If 'true' is
	// passed, it also announces it, otherwise it is just kept in the local
	// accounting of which objects are being provided.
	Provider(string, bool) error
}
