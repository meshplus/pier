package syncer

import (
	"github.com/meshplus/bitxhub-kit/storage"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

type Config struct {
	client    rpcx.Client
	ethClient *Client
	lite      lite.Lite
	storage   storage.Storage
	config    *repo.Config
	logger    logrus.FieldLogger
}

type Option func(*Config)

func WithClient(cli rpcx.Client) Option {
	return func(c *Config) {
		c.client = cli
	}
}

func WithEthClient(cli *Client) Option {
	return func(c *Config) {
		c.ethClient = cli
	}
}

func WithLite(lite lite.Lite) Option {
	return func(c *Config) {
		c.lite = lite
	}
}

func WithConfig(config *repo.Config) Option {
	return func(c *Config) {
		c.config = config
	}
}

func WithStorage(store storage.Storage) Option {
	return func(config *Config) {
		config.storage = store
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(config *Config) {
		config.logger = logger
	}
}

func GenerateConfig(opts ...Option) (*Config, error) {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	return config, nil
}
