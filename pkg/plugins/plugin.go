package plugins

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-model/pb"
	"google.golang.org/grpc"
)

// Handshake is a common handshake that is shared by plugin and host.
var (
	Handshake = plugin.HandshakeConfig{
		ProtocolVersion:  4,
		MagicCookieKey:   "PIER_APPCHAIN_PLUGIN",
		MagicCookieValue: "PIER",
	}
	PluginName = "appchain-plugin"
)

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	PluginName: &AppchainGRPCPlugin{},
}

// This is the implementation of plugin.GRPCPlugin so we can serve/consume this.
type AppchainGRPCPlugin struct {
	plugin.Plugin
	// Concrete implementation, written in Go. This is only used for plugins
	// that are written in Go.
	Impl Client
}

func (p *AppchainGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterAppchainPluginServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *AppchainGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{
		client:      pb.NewAppchainPluginClient(c),
		doneContext: ctx,
	}, nil
}
