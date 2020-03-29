package plugins

import (
	"fmt"
	"path/filepath"
	"plugin"

	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/pkg/plugins/client"
)

// CreateClient creates client instance interacting with appchain by loading dynamic loaded file
func CreateClient(id string, config *repo.Config, extra []byte) (client.Client, error) {
	pluginRoot, err := repo.PluginPath()
	if err != nil {
		return nil, err
	}
	pluginPath := filepath.Join(pluginRoot, config.Appchain.Plugin)
	configPath := filepath.Join(config.RepoRoot, config.Appchain.Config)

	p, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("plugin open error: %s", err)
	}

	m, err := p.Lookup("NewClient")
	if err != nil {
		return nil, fmt.Errorf("plugin lookup error: %s", err)
	}

	newC, ok := m.(func(string, string, []byte) (client.Client, error))
	if !ok {
		return nil, fmt.Errorf("assert NewClient error")
	}

	cli, err := newC(configPath, id, extra)
	if err != nil {
		return nil, err
	}

	return cli, nil
}
