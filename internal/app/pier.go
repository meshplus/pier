package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/exchanger"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/lite/bxh_lite"
	"github.com/meshplus/pier/internal/lite/direct_lite"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/rulemgr"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	plugin "github.com/meshplus/pier/pkg/plugins/client"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("app")

// Pier represents the necessary data for starting the pier app
type Pier struct {
	privateKey crypto.PrivateKey
	plugin     plugin.Client
	monitor    monitor.Monitor
	exec       executor.Executor
	lite       lite.Lite
	storage    storage.Storage
	exchanger  exchanger.IExchanger
	ctx        context.Context
	cancel     context.CancelFunc
	appchain   *rpcx.Appchain
	meta       *rpcx.Interchain
}

// NewPier instantiates pier instance.
func NewPier(repoRoot string, config *repo.Config) (*Pier, error) {
	store, err := leveldb.New(filepath.Join(config.RepoRoot, "store"))
	if err != nil {
		return nil, fmt.Errorf("read from datastaore %w", err)
	}

	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	addr, err := privateKey.PublicKey().Address()
	if err != nil {
		return nil, fmt.Errorf("get address from private key %w", err)
	}

	var (
		ag          agent.Agent
		ck          checker.Checker
		cryptor     txcrypto.Cryptor
		ex          exchanger.IExchanger
		lite        lite.Lite
		sync        syncer.Syncer
		apiServer   *api.Server
		meta        *rpcx.Interchain
		chain       *rpcx.Appchain
		peerManager peermgr.PeerManager
	)

	switch config.Mode.Type {
	case repo.DirectMode:
		peerManager, err = peermgr.New(config, privateKey)
		if err != nil {
			return nil, fmt.Errorf("peerMgr create: %w", err)
		}

		ruleMgr, err := rulemgr.New(store, peerManager)
		if err != nil {
			return nil, fmt.Errorf("ruleMgr create: %w", err)
		}

		appchainMgr, err := appchain.NewManager(addr.String(), store, peerManager)
		if err != nil {
			return nil, fmt.Errorf("ruleMgr create: %w", err)
		}

		apiServer, err = api.NewServer(appchainMgr, peerManager, config)
		if err != nil {
			return nil, fmt.Errorf("gin service create: %w", err)
		}

		ck = checker.NewDirectChecker(ruleMgr, appchainMgr)

		cryptor, err = txcrypto.NewDirectCryptor(appchainMgr, privateKey)
		if err != nil {
			return nil, fmt.Errorf("cryptor create: %w", err)
		}

		meta = &rpcx.Interchain{}
		lite = &direct_lite.MockLite{}
	case repo.RelayMode:
		ck = &checker.MockChecker{}

		// pier register to bitxhub and got meta infos about its related
		// appchain from bitxhub
		client, err := rpcx.New(
			rpcx.WithAddrs([]string{config.Mode.Relay.Addr}),
			rpcx.WithLogger(logger),
			rpcx.WithPrivateKey(privateKey),
		)
		if err != nil {
			return nil, fmt.Errorf("create bitxhub client: %w", err)
		}

		ag, err = agent.New(client, addr, config.Mode.Relay)
		if err != nil {
			return nil, fmt.Errorf("create agent error: %w", err)
		}

		// agent queries appchain info from bitxhub
		meta, err = ag.GetInterchainMeta()
		if err != nil {
			return nil, err
		}

		chain, err = ag.Appchain()
		if err != nil {
			return nil, err
		}

		cryptor, err = txcrypto.NewRelayCryptor(client, privateKey)
		if err != nil {
			return nil, fmt.Errorf("cryptor create: %w", err)
		}

		lite, err = bxh_lite.New(ag, store)
		if err != nil {
			return nil, fmt.Errorf("lite create: %w", err)
		}

		sync, err = syncer.New(ag, lite, store)
		if err != nil {
			return nil, fmt.Errorf("syncer create: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported mode")
	}

	//use meta info to instantiate monitor and executor module
	extra, err := json.Marshal(meta.InterchainCounter)
	if err != nil {
		return nil, fmt.Errorf("marshal interchain meta: %w", err)
	}

	cli, err := plugins.CreateClient(addr.String(), config, extra)
	if err != nil {
		return nil, fmt.Errorf("appchain client create: %w", err)
	}

	mnt, err := monitor.New(cli, cryptor)
	if err != nil {
		return nil, fmt.Errorf("monitor create: %w", err)
	}

	exec, err := executor.New(cli, addr.String(), store, cryptor)
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	ex, err = exchanger.New(config.Mode.Type, addr.String(), meta,
		exchanger.WithAgent(ag),
		exchanger.WithChecker(ck),
		exchanger.WithExecutor(exec),
		exchanger.WithMonitor(mnt),
		exchanger.WithPeerMgr(peerManager),
		exchanger.WithSyncer(sync),
		exchanger.WithAPIServer(apiServer),
		exchanger.WithStorage(store),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey: privateKey,
		plugin:     cli,
		appchain:   chain,
		meta:       meta,
		monitor:    mnt,
		exchanger:  ex,
		exec:       exec,
		lite:       lite,
		storage:    store,
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

	if err := pier.lite.Start(); err != nil {
		return fmt.Errorf("lite start: %w", err)
	}

	if err := pier.exchanger.Start(); err != nil {
		return fmt.Errorf("exchanger start: %w", err)
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

	if err := pier.lite.Stop(); err != nil {
		return fmt.Errorf("lite stop: %w", err)
	}

	if err := pier.exchanger.Stop(); err != nil {
		return fmt.Errorf("exchanger stop: %w", err)
	}
	return nil
}

// Type gets the application blockchain type the pier is related to
func (pier *Pier) Type() string {
	return pier.plugin.Type()
}
