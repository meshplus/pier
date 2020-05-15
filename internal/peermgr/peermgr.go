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
	// Start
	Start() error

	// Stop
	Stop() error

	// AsyncSend sends message to peer with peer info.
	AsyncSend(string, *peermgr.Message) error

	// SendWithStream sends message using existed stream
	SendWithStream(network.Stream, *peermgr.Message) error

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
