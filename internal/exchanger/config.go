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
)

type Config struct {
	agent   agent.Agent
	checker checker.Checker
	store   storage.Storage
	peerMgr peermgr.PeerManager
	mnt     monitor.Monitor
	exec    executor.Executor
	syncer  syncer.Syncer
	gin     api.GinService
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

func WithGin(gin api.GinService) Option {
	return func(config *Config) {
		config.gin = gin
	}
}

func GenerateConfig(opts ...Option) (*Config, error) {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	return config, nil
}
