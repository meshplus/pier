package syncer

import (
	"github.com/meshplus/bitxhub-kit/storage"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/repo"
)

type Config struct {
	client  rpcx.Client
	lite    lite.Lite
	storage storage.Storage
	config  *repo.Config
}

type Option func(*Config)

func WithClient(cli rpcx.Client) Option {
	return func(c *Config) {
		c.client = cli
	}
}

func WithLite(lite lite.Lite) Option {
	return func(c *Config) {
		c.lite = lite
	}
}

func WithStorage(store storage.Storage) Option {
	return func(config *Config) {
		config.storage = store
	}
}

func GenerateConfig(opts ...Option) (*Config, error) {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	return config, nil
}
