package plugins

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/pier/internal/repo"
)

var logger = hclog.New(&hclog.LoggerOptions{
	Name:   "plugin",
	Output: hclog.DefaultOutput,
	Level:  hclog.Trace,
})

func CreateClient(appchainConfig *repo.Appchain, extra []byte) (Client, *plugin.Client, error) {
	// Pier is the host. Start by launching the plugin process.
	rootPath, err := repo.PathRoot()
	if err != nil {
		return nil, nil, err
	}
	pluginConfigPath := filepath.Join(rootPath, appchainConfig.Config)
	pluginPath := filepath.Join(rootPath, "plugins", appchainConfig.Plugin)

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command("sh", "-c", fmt.Sprintf("%s start", pluginPath)),
		Logger:          logger,
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("connect via rpc: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		return nil, nil, fmt.Errorf("dispense plugin %s: %w", PluginName, err)
	}

	var appchain Client
	switch raw.(type) {
	case *GRPCClient:
		appchain = raw.(*GRPCClient)
	default:
		return nil, nil, fmt.Errorf("unsupported client type")
	}

	// initialize our client plugin
	err = appchain.Initialize(pluginConfigPath, extra)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize plugin %s: %w", pluginConfigPath, err)
	}

	return appchain, client, nil
}
