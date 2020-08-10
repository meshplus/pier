package peermgr

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	peermgr "github.com/meshplus/pier/internal/peermgr/proto"
)

type MessageHandler func(network.Stream, *peermgr.Message)
type ConnectHandler func(string)

//go:generate mockgen -destination mock_peermgr/mock_peermgr.go -package mock_peermgr -source peermgr.go
type PeerManager interface {
	DHTManager

	// Start
	Start() error

	// Stop
	Stop() error

	// AsyncSend sends message to peer with peer info.
	AsyncSend(string, *peermgr.Message) error

	Connect(info *peer.AddrInfo) error

	// SendWithStream sends message using existed stream
	SendWithStream(network.Stream, *peermgr.Message) (*peermgr.Message, error)

	// AsyncSendWithStream sends message using existed stream
	AsyncSendWithStream(network.Stream, *peermgr.Message) error

	// Send sends message waiting response
	Send(string, *peermgr.Message) (*peermgr.Message, error)

	// Peers
	Peers() map[string]*peer.AddrInfo

	// RegisterMsgHandler
	RegisterMsgHandler(peermgr.Message_Type, MessageHandler) error

	// RegisterMultiMsgHandler
	RegisterMultiMsgHandler([]peermgr.Message_Type, MessageHandler) error

	// RegisterConnectHandler
	RegisterConnectHandler(ConnectHandler) error
}

type DHTManager interface {
	// Search for peers who are able to provide a given key
	// When count is 0, this method will return an unbounded number of
	// results.
	FindProviders(string, int) ([]peer.AddrInfo, error)

	// Provide adds the given cid to the content routing system. If 'true' is
	// passed, it also announces it, otherwise it is just kept in the local
	// accounting of which objects are being provided.
	Provider(string, bool) error
}
