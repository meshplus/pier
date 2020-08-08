package exchanger

import (
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/sirupsen/logrus"
)

type Config struct {
	agent     agent.Agent
	checker   checker.Checker
	store     storage.Storage
	peerMgr   peermgr.PeerManager
	mnt       monitor.Monitor
	exec      executor.Executor
	syncer    syncer.Syncer
	apiServer *api.Server
	logger    logrus.FieldLogger
}

type Option func(*Config)

func WithAgent(ag agent.Agent) Option {
	return func(config *Config) {
		config.agent = ag
	}
}

func WithChecker(checker checker.Checker) Option {
	return func(config *Config) {
		config.checker = checker
	}
}

func WithExecutor(exec executor.Executor) Option {
	return func(config *Config) {
		config.exec = exec
	}
}

func WithMonitor(mnt monitor.Monitor) Option {
	return func(config *Config) {
		config.mnt = mnt
	}
}

func WithPeerMgr(mgr peermgr.PeerManager) Option {
	return func(config *Config) {
		config.peerMgr = mgr
	}
}

func WithSyncer(syncer syncer.Syncer) Option {
	return func(config *Config) {
		config.syncer = syncer
	}
}

func WithAPIServer(apiServer *api.Server) Option {
	return func(config *Config) {
		config.apiServer = apiServer
	}
}

func WithStorage(store storage.Storage) Option {
	return func(config *Config) {
		config.store = store
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
