package plugins

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/pier/internal/repo"
	appchainPlugin "github.com/meshplus/pier/pkg/plugins/client"
)

func CreateClient(pierID string, config *repo.Config, extra []byte) (appchainPlugin.Client, error) {
	// We don't want to see the plugin logs.
	log.SetOutput(ioutil.Discard)

	// We're a host. Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: appchainPlugin.Handshake,
		Plugins:         appchainPlugin.PluginMap,
		Cmd:             exec.Command("sh", "-c", os.Getenv("APPCHAIN_PLUGIN")),
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
	raw, err := rpcClient.Dispense(config.Appchain.Plugin)
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	var appchain appchainPlugin.Client
	switch raw.(type) {
	case *appchainPlugin.GRPCClient:
		appchain = raw.(*appchainPlugin.GRPCClient)
	default:
		return nil, fmt.Errorf("unsupported client type")
	}

	// initialize our client plugin
	err = appchain.Initialize(config.Appchain.Config, pierID, extra)
	if err != nil {
		return nil, err
	}

	return appchain, nil
}
