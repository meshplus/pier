package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/pier/internal/repo"
)

func CreateClient(pierID string, config *repo.Config, extra []byte) (Client, error) {
	// Pier is the host. Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command("sh", "-c", "plugins/appchain_plugin"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolNetRPC, plugin.ProtocolGRPC},
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	var appchain Client
	switch raw.(type) {
	case *GRPCClient:
		appchain = raw.(*GRPCClient)
	default:
		return nil, fmt.Errorf("unsupported client type")
	}

	// initialize our client plugin
	rootPath, err := repo.PathRoot()
	pluginConfigPath := filepath.Join(rootPath, config.Appchain.Config)
	err = appchain.Initialize(pluginConfigPath, pierID, extra)
	if err != nil {
		return nil, err
	}

	return appchain, nil
}
