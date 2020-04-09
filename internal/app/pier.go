package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/pkg/plugins"
	plugin "github.com/meshplus/pier/pkg/plugins/client"
)

var logger = log.NewWithModule("app")

// Pier represents the necessary data for starting the pier app
type Pier struct {
	privateKey crypto.PrivateKey
	plugin     plugin.Client
	agent      agent.Agent
	monitor    monitor.Monitor
	exec       executor.Executor
	sync       syncer.Syncer
	storage    storage.Storage
	ctx        context.Context
	cancel     context.CancelFunc
	meta       *rpcx.Appchain
}

// NewPier instantiates pier instance.
func NewPier(repoRoot string, config *repo.Config) (*Pier, error) {
	storage, err := leveldb.New(filepath.Join(config.RepoRoot, "storage"))
	if err != nil {
		return nil, fmt.Errorf("read from datastaore %w", err)
	}

	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	// pier register to bitxhub and got meta infos about its related
	// appchain from bitxhub
	client, err := rpcx.New(
		rpcx.WithAddrs([]string{config.Bitxhub.Addr}),
		rpcx.WithLogger(logger),
		rpcx.WithPrivateKey(privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("create bitxhub client: %w", err)
	}

	addr, err := privateKey.PublicKey().Address()
	if err != nil {
		return nil, fmt.Errorf("get address from private key %w", err)
	}

	ag, err := agent.New(client, addr, config.Bitxhub)
	if err != nil {
		return nil, fmt.Errorf("create agent error: %w", err)
	}

	// agent query appchain info from bitxhub
	chain, err := ag.Appchain()
	if err != nil {
		return nil, err
	}

	//use chain info to instantiate monitor and executor module
	extra, err := json.Marshal(chain.InterchainCounter)
	if err != nil {
		return nil, fmt.Errorf("marshal interchain meta: %w", err)
	}

	cli, err := plugins.CreateClient(addr.String(), config, extra)
	if err != nil {
		return nil, fmt.Errorf("client create: %w", err)
	}

	mnt, err := monitor.New(ag, cli, chain)
	if err != nil {
		return nil, fmt.Errorf("monitor create: %w", err)
	}

	exec, err := executor.NewChannelExecutor(ag, cli, chain, storage)
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	sync, err := syncer.New(ag, config.Bitxhub.Quorum, config.Bitxhub.GetValidators(), storage)
	if err != nil {
		return nil, fmt.Errorf("syncer create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey: privateKey,
		plugin:     cli,
		agent:      ag,
		meta:       chain,
		monitor:    mnt,
		exec:       exec,
		sync:       sync,
		storage:    storage,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts three main components of pier app
func (pier *Pier) Start() error {
	logger.WithFields(logrus.Fields{
		"id":                     pier.meta.ID,
		"interchain_counter":     pier.meta.InterchainCounter,
		"receipt_counter":        pier.meta.ReceiptCounter,
		"source_receipt_counter": pier.meta.SourceReceiptCounter,
	}).Info("Pier information")

	if err := pier.monitor.Start(); err != nil {
		return fmt.Errorf("monitor start: %w", err)
	}

	if err := pier.exec.Start(); err != nil {
		return fmt.Errorf("executor start: %w", err)
	}

	if err := pier.sync.Start(); err != nil {
		return fmt.Errorf("sync start: %w", err)
	}

	return nil
}

// Stop stops three main components of pier app
func (pier *Pier) Stop() error {
	if err := pier.monitor.Stop(); err != nil {
		return fmt.Errorf("monitor stop: %w", err)
	}

	if err := pier.exec.Stop(); err != nil {
		return fmt.Errorf("executor stop: %w", err)
	}

	if err := pier.sync.Stop(); err != nil {
		return fmt.Errorf("sync stop: %w", err)
	}

	return nil
}

// Type gets the application blockchain type the pier is related to
func (pier *Pier) Type() string {
	return pier.plugin.Type()
}
