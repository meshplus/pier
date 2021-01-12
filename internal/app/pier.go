package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/exchanger"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/lite/bxh_lite"
	"github.com/meshplus/pier/internal/lite/direct_lite"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/router"
	"github.com/meshplus/pier/internal/rulemgr"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

// Pier represents the necessary data for starting the pier app
type Pier struct {
	privateKey crypto.PrivateKey
	plugin     plugins.Client
	grpcPlugin *plugin.Client
	monitor    monitor.Monitor
	exec       executor.Executor
	lite       lite.Lite
	storage    storage.Storage
	exchanger  exchanger.IExchanger
	ctx        context.Context
	cancel     context.CancelFunc
	appchain   *rpcx.Appchain
	meta       *pb.Interchain
	config     *repo.Config
	logger     logrus.FieldLogger
}

// NewPier instantiates pier instance.
func NewPier(repoRoot string, config *repo.Config) (*Pier, error) {
	store, err := leveldb.New(filepath.Join(config.RepoRoot, "store"))
	if err != nil {
		return nil, fmt.Errorf("read from datastaore %w", err)
	}

	logger := loggers.Logger(loggers.App)
	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	addr, err := privateKey.PublicKey().Address()
	if err != nil {
		return nil, fmt.Errorf("get address from private key %w", err)
	}

	nodePrivKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load node key: %w", err)
	}

	var (
		ck          checker.Checker
		cryptor     txcrypto.Cryptor
		ex          exchanger.IExchanger
		lite        lite.Lite
		sync        syncer.Syncer
		apiServer   *api.Server
		meta        *pb.Interchain
		chain       *rpcx.Appchain
		peerManager peermgr.PeerManager
	)

	switch config.Mode.Type {
	case repo.DirectMode:
		peerManager, err = peermgr.New(config, nodePrivKey, privateKey, 1, loggers.Logger(loggers.PeerMgr))
		if err != nil {
			return nil, fmt.Errorf("peerMgr create: %w", err)
		}

		ruleMgr, err := rulemgr.New(store, peerManager, loggers.Logger(loggers.RuleMgr))
		if err != nil {
			return nil, fmt.Errorf("ruleMgr create: %w", err)
		}

		appchainMgr, err := appchain.NewManager(addr.String(), store, peerManager, loggers.Logger(loggers.AppchainMgr))
		if err != nil {
			return nil, fmt.Errorf("ruleMgr create: %w", err)
		}

		apiServer, err = api.NewServer(appchainMgr, peerManager, config, loggers.Logger(loggers.ApiServer))
		if err != nil {
			return nil, fmt.Errorf("gin service create: %w", err)
		}

		ck = checker.NewDirectChecker(ruleMgr, appchainMgr)

		cryptor, err = txcrypto.NewDirectCryptor(appchainMgr, privateKey)
		if err != nil {
			return nil, fmt.Errorf("cryptor create: %w", err)
		}

		meta = &pb.Interchain{}
		lite = &direct_lite.MockLite{}
	case repo.RelayMode:
		ck = &checker.MockChecker{}

		// pier register to bitxhub and got meta infos about its related
		// appchain from bitxhub
		opts := []rpcx.Option{
			rpcx.WithLogger(logger),
			rpcx.WithPrivateKey(privateKey),
		}
		nodeInfo := &rpcx.NodeInfo{Addr: config.Mode.Relay.Addr}
		if config.Security.EnableTLS {
			nodeInfo.CertPath = filepath.Join(config.RepoRoot, config.Security.Tlsca)
			nodeInfo.EnableTLS = config.Security.EnableTLS
			nodeInfo.CommonName = config.Security.CommonName
		}
		opts = append(opts, rpcx.WithNodesInfo(nodeInfo))
		client, err := rpcx.New(opts...)
		if err != nil {
			return nil, fmt.Errorf("create bitxhub client: %w", err)
		}

		// agent queries appchain info from bitxhub
		meta, err = getInterchainMeta(client)
		if err != nil {
			return nil, err
		}

		chain, err = getAppchainInfo(client)
		if err != nil {
			return nil, err
		}

		cryptor, err = txcrypto.NewRelayCryptor(client, privateKey)
		if err != nil {
			return nil, fmt.Errorf("cryptor create: %w", err)
		}

		lite, err = bxh_lite.New(client, store, loggers.Logger(loggers.BxhLite))
		if err != nil {
			return nil, fmt.Errorf("lite create: %w", err)
		}

		sync, err = syncer.New(addr.String(), repo.RelayMode,
			syncer.WithClient(client), syncer.WithLite(lite),
			syncer.WithStorage(store), syncer.WithLogger(loggers.Logger(loggers.Syncer)),
		)
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

	var cli plugins.Client
	var grpcPlugin *plugin.Client
	err = retry.Retry(func(attempt uint) error {
		cli, grpcPlugin, err = plugins.CreateClient(addr.String(), config.Appchain.Config, extra)
		if err != nil {
			logger.Errorf("client plugin create:%s", err)
		}
		return err
	}, strategy.Wait(3*time.Second))
	if err != nil {
		logger.Panic(err)
	}

	mnt, err := monitor.New(cli, cryptor, loggers.Logger(loggers.Monitor))
	if err != nil {
		return nil, fmt.Errorf("monitor create: %w", err)
	}

	exec, err := executor.New(cli, addr.String(), store, cryptor, loggers.Logger(loggers.Executor))
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	ex, err = exchanger.New(config.Mode.Type, addr.String(), meta,
		exchanger.WithChecker(ck),
		exchanger.WithExecutor(exec),
		exchanger.WithMonitor(mnt),
		exchanger.WithPeerMgr(peerManager),
		exchanger.WithSyncer(sync),
		exchanger.WithAPIServer(apiServer),
		exchanger.WithStorage(store),
		exchanger.WithLogger(loggers.Logger(loggers.Exchanger)),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey: privateKey,
		plugin:     cli,
		grpcPlugin: grpcPlugin,
		appchain:   chain,
		meta:       meta,
		monitor:    mnt,
		exchanger:  ex,
		exec:       exec,
		lite:       lite,
		storage:    store,
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
	}, nil
}

func NewUnionPier(repoRoot string, config *repo.Config) (*Pier, error) {
	store, err := leveldb.New(filepath.Join(config.RepoRoot, "store"))
	if err != nil {
		return nil, fmt.Errorf("read from datastaore %w", err)
	}

	logger := loggers.Logger(loggers.App)
	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	addr, err := privateKey.PublicKey().Address()
	if err != nil {
		return nil, fmt.Errorf("get address from private key %w", err)
	}
	var (
		ex          exchanger.IExchanger
		lite        lite.Lite
		sync        syncer.Syncer
		meta        *pb.Interchain
		chain       *rpcx.Appchain
		peerManager peermgr.PeerManager
	)

	nodePrivKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load node key: %w", err)
	}

	peerManager, err = peermgr.New(config, nodePrivKey, privateKey, config.Mode.Union.Providers, loggers.Logger(loggers.PeerMgr))
	if err != nil {
		return nil, fmt.Errorf("peerMgr create: %w", err)
	}

	opts := []rpcx.Option{
		rpcx.WithLogger(logger),
		rpcx.WithPrivateKey(privateKey),
	}
	nodeInfo := &rpcx.NodeInfo{Addr: config.Mode.Relay.Addr}
	if config.Security.EnableTLS {
		nodeInfo.CertPath = filepath.Join(config.RepoRoot, config.Security.Tlsca)
		nodeInfo.EnableTLS = config.Security.EnableTLS
		nodeInfo.CommonName = config.Security.CommonName
	}
	opts = append(opts, rpcx.WithNodesInfo(nodeInfo))
	client, err := rpcx.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create bitxhub client: %w", err)
	}

	meta = &pb.Interchain{}

	lite, err = bxh_lite.New(client, store, loggers.Logger(loggers.BxhLite))
	if err != nil {
		return nil, fmt.Errorf("lite create: %w", err)
	}

	sync, err = syncer.New(addr.String(), repo.UnionMode,
		syncer.WithClient(client), syncer.WithLite(lite), syncer.WithStorage(store),
	)
	if err != nil {
		return nil, fmt.Errorf("syncer create: %w", err)
	}

	cli := agent.CreateClient(ag)
	exec, err := executor.New(cli, addr.String(), store, nil)
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	router := router.New(peerManager, store, peerManager.(*peermgr.Swarm).ConnectedPeerIDs())

	ex, err = exchanger.New(config.Mode.Type, addr.String(), meta,
		exchanger.WithAgent(ag),
		exchanger.WithExecutor(exec),
		exchanger.WithPeerMgr(peerManager),
		exchanger.WithSyncer(sync),
		exchanger.WithStorage(store),
		exchanger.WithRouter(router),
		exchanger.WithLogger(loggers.Logger(loggers.Exchanger)),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey: privateKey,
		appchain:   chain,
		meta:       meta,
		exchanger:  ex,
		exec:       exec,
		lite:       lite,
		storage:    store,
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
	}, nil
}

// Start starts three main components of pier app
func (pier *Pier) Start() error {
	if pier.config.Mode.Type != repo.UnionMode {
		pier.logger.WithFields(logrus.Fields{
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
	if pier.config.Mode.Type != repo.UnionMode {
		if err := pier.monitor.Stop(); err != nil {
			return fmt.Errorf("monitor stop: %w", err)
		}

		// stop appchain plugin first and kill plugin process
		pier.grpcPlugin.Kill()

		if err := pier.exec.Stop(); err != nil {
			return fmt.Errorf("executor stop: %w", err)
		}
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
	if pier.config.Mode.Type != repo.UnionMode {
		return pier.plugin.Type()
	}
	return repo.UnionMode
}
