package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/pier/internal/repo"
)

var logger = hclog.New(&hclog.LoggerOptions{
	Name:   "plugin",
	Output: os.Stdout,
	Level:  hclog.Info,
})

//var logger = log.NewWithModule("plugin")

func CreateClient(pierID string, config *repo.Config, extra []byte) (Client, error) {
	// Pier is the host. Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command("sh", "-c", "plugins/appchain_plugin"),
		Logger:          logger,
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
	if err != nil {
		return nil, err
	}
	pluginConfigPath := filepath.Join(rootPath, config.Appchain.Config)
	err = appchain.Initialize(pluginConfigPath, pierID, extra)
	if err != nil {
		return nil, err
	}

	return appchain, nil
}
