package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/meshplus/pier/internal/adapt/bxh_adapter"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-core/agency"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/api"
	_ "github.com/meshplus/pier/imports"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/exchanger"
	exchanger2 "github.com/meshplus/pier/internal/exchanger2"
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
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	_ "github.com/meshplus/pier/pkg/single"
	"github.com/sirupsen/logrus"
	"github.com/wonderivan/logger"
)

// Pier represents the necessary data for starting the pier app
type Pier struct {
	privateKey  crypto.PrivateKey
	plugin      plugins.Client
	grpcPlugin  *plugin.Client
	monitor     monitor.Monitor
	exec        executor.Executor
	lite        lite.Lite
	pierHA      agency.PierHA
	storage     storage.Storage
	exchanger   exchanger.IExchanger
	ctx         context.Context
	cancel      context.CancelFunc
	appchain    *appchainmgr.Appchain
	serviceMeta map[string]*pb.Interchain
	config      *repo.Config
	logger      logrus.FieldLogger
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

	//addr, err := privateKey.PublicKey().Address()
	//if err != nil {
	//	return nil, fmt.Errorf("get address from private key %w", err)
	//}

	nodePrivKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load node key: %w", err)
	}

	var (
		ck        checker.Checker
		cryptor   txcrypto.Cryptor
		ex        exchanger.IExchanger
		lite      lite.Lite
		pierHA    agency.PierHA
		apiServer *api.Server
		//chain       *appchainmgr.Appchain
		peerManager peermgr.PeerManager
		bxhID       string
		serviceMeta = make(map[string]*pb.Interchain)
	)

	switch config.Mode.Type {
	case repo.DirectMode:
		peerManager, err = peermgr.New(config, nodePrivKey, privateKey, 1, loggers.Logger(loggers.PeerMgr))
		if err != nil {
			return nil, fmt.Errorf("peerMgr create: %w", err)
		}

		ruleMgr, err := rulemgr.New(store, peerManager, loggers.Logger(loggers.RuleMgr), config.Mode.Direct.GasLimit)
		if err != nil {
			return nil, fmt.Errorf("ruleMgr create: %w", err)
		}

		appchainMgr, err := appchain.NewManager(config.Appchain.ID, store, peerManager, loggers.Logger(loggers.AppchainMgr))
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

		lite = &direct_lite.MockLite{}
	case repo.RelayMode:
		ck = &checker.MockChecker{}

		client, err := newBitXHubClient(logger, privateKey, config)
		if err != nil {
			return nil, fmt.Errorf("create bitxhub client: %w", err)
		}

		bxhID = config.Mode.Relay.BitXHubID

		cryptor, err = txcrypto.NewRelayCryptor(client, privateKey)
		if err != nil {
			return nil, fmt.Errorf("cryptor create: %w", err)
		}

		lite, err = bxh_lite.New(client, store, loggers.Logger(loggers.BxhLite))
		if err != nil {
			return nil, fmt.Errorf("lite create: %w", err)
		}

		pierHAConstructor, err := agency.GetPierHAConstructor(config.HA.Mode)
		if err != nil {
			return nil, fmt.Errorf("pier ha constructor not found")
		}

		pierHA = pierHAConstructor(client, config.Appchain.ID)
	default:
		return nil, fmt.Errorf("unsupported mode")
	}

	//use meta info to instantiate monitor and executor module
	extra, err := json.Marshal(serviceMeta)
	if err != nil {
		return nil, fmt.Errorf("marshal interchain meta: %w", err)
	}

	var cli plugins.Client
	var grpcPlugin *plugin.Client
	err = retry.Retry(func(attempt uint) error {
		cli, grpcPlugin, err = plugins.CreateClient(config.Appchain.ID, config.Appchain, extra)
		if err != nil {
			logger.Errorf("client plugin create:%s", err)
		}
		return err
	}, strategy.Wait(3*time.Second))
	if err != nil {
		logger.Panic(err)
	}

	services, err := cli.GetServices()
	if err != nil {
		return nil, err
	}

	serviceMeta = filterServiceMeta(serviceMeta, bxhID, config.Appchain.ID, services)

	mnt, err := monitor.New(cli, cryptor, loggers.Logger(loggers.Monitor))
	if err != nil {
		return nil, fmt.Errorf("monitor create: %w", err)
	}

	exec, err := executor.New(cli, config.Appchain.ID, store, cryptor, loggers.Logger(loggers.Executor))
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	ex, err = exchanger.New(config.Mode.Type, config.Appchain.ID, serviceMeta,
		exchanger.WithChecker(ck),
		exchanger.WithExecutor(exec),
		exchanger.WithMonitor(mnt),
		exchanger.WithPeerMgr(peerManager),
		exchanger.WithAPIServer(apiServer),
		exchanger.WithStorage(store),
		exchanger.WithLogger(loggers.Logger(loggers.Exchanger)),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey:  privateKey,
		plugin:      cli,
		grpcPlugin:  grpcPlugin,
		serviceMeta: serviceMeta,
		monitor:     mnt,
		exchanger:   ex,
		exec:        exec,
		lite:        lite,
		pierHA:      pierHA,
		logger:      logger,
		storage:     store,
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
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
		ck          checker.Checker
		peerManager peermgr.PeerManager
		serviceMeta = make(map[string]*pb.Interchain)
	)

	ck = &checker.MockChecker{}
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
	nodesInfo := make([]*rpcx.NodeInfo, 0, len(config.Mode.Union.Addrs))
	for index, addr := range config.Mode.Union.Addrs {
		nodeInfo := &rpcx.NodeInfo{Addr: addr}
		if config.Security.EnableTLS {
			nodeInfo.CertPath = filepath.Join(config.RepoRoot, config.Security.Tlsca)
			nodeInfo.EnableTLS = config.Security.EnableTLS
			nodeInfo.CommonName = config.Security.CommonName
			nodeInfo.AccessCert = filepath.Join(config.RepoRoot, config.Security.AccessCert[index])
			nodeInfo.AccessKey = filepath.Join(config.RepoRoot, config.Security.AccessKey)
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	opts = append(opts, rpcx.WithNodesInfo(nodesInfo...), rpcx.WithTimeoutLimit(config.Mode.Relay.TimeoutLimit))
	client, err := rpcx.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create bitxhub client: %w", err)
	}

	lite, err = bxh_lite.New(client, store, loggers.Logger(loggers.BxhLite))
	if err != nil {
		return nil, fmt.Errorf("lite create: %w", err)
	}

	cli := agent.CreateClient(client)
	exec, err := executor.New(cli, addr.String(), store, nil, loggers.Logger(loggers.Executor))
	if err != nil {
		return nil, fmt.Errorf("executor create: %w", err)
	}

	router := router.New(peerManager, store, loggers.Logger(loggers.Router), peerManager.(*peermgr.Swarm).ConnectedPeerIDs())

	ex, err = exchanger.New(config.Mode.Type, addr.String(), serviceMeta,
		exchanger.WithExecutor(exec),
		exchanger.WithPeerMgr(peerManager),
		exchanger.WithChecker(ck),
		exchanger.WithStorage(store),
		exchanger.WithRouter(router),
		exchanger.WithLogger(loggers.Logger(loggers.Exchanger)),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey:  privateKey,
		serviceMeta: serviceMeta,
		exchanger:   ex,
		exec:        exec,
		lite:        lite,
		storage:     store,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
	}, nil
}

// Start starts three main components of pier app
func (pier *Pier) Start() error {
	if pier.config.Mode.Type == repo.UnionMode {
		if err := pier.lite.Start(); err != nil {
			pier.logger.Errorf("lite start: %w", err)
			return err
		}
		if err := pier.exchanger.Start(); err != nil {
			pier.logger.Errorf("exchanger start: %w", err)
			return err
		}
		return nil
	}
	if err := pier.pierHA.Start(); err != nil {
		return fmt.Errorf("pier ha start fail")
	}
	go pier.startPierHA()
	return nil
}

func NewPier2(repoRoot string, config *repo.Config) (*Pier, error) {
	var (
		ex     exchanger.IExchanger
		pierHA agency.PierHA
	)

	logger := loggers.Logger(loggers.App)
	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	switch config.Mode.Type {
	case repo.DirectMode:
		return nil, fmt.Errorf("direct mode is unsupported yet")
	case repo.RelayMode:
		appchainAdapter, err := adapt.NewAppchainAdapter(config, loggers.Logger(loggers.Appchain))
		if err != nil {
			return nil, fmt.Errorf("new appchain adapter: %w", err)
		}

		client, err := newBitXHubClient(logger, privateKey, config)
		if err != nil {
			return nil, fmt.Errorf("create bitxhub client: %w", err)
		}

		bxhAdapter, err := bxh_adapter.New(config.Appchain.ID, config.Appchain.ID, repo.RelayMode,
			bxh_adapter.WithClient(client),
			bxh_adapter.WithLogger(loggers.Logger(loggers.Syncer)))
		if err != nil {
			return nil, fmt.Errorf("new bitxhub adapter: %w", err)
		}

		pierHAConstructor, err := agency.GetPierHAConstructor(config.HA.Mode)
		if err != nil {
			return nil, fmt.Errorf("pier ha constructor not found")
		}

		pierHA = pierHAConstructor(client, config.Appchain.ID)

		ex, err = exchanger2.New(repo.RelayMode, config.Appchain.ID, config.Mode.Relay.BitXHubID,
			exchanger2.WithSrcAdapt(appchainAdapter),
			exchanger2.WithDestAdapt(bxhAdapter),
			exchanger2.WithLogger(log.NewWithModule("exchanger")))
		if err != nil {
			return nil, fmt.Errorf("exchanger create: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported mode")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pier{
		privateKey: privateKey,
		exchanger:  ex,
		pierHA:     pierHA,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
	}, nil
}

func (pier *Pier) startPierHA() {
	logger.Info("pier HA manager start")
	status := false
	for {
		select {
		case isMain := <-pier.pierHA.IsMain():
			if isMain {
				if status {
					continue
				}
				for serviceID, meta := range pier.serviceMeta {
					pier.logger.WithFields(logrus.Fields{
						"id":                        serviceID,
						"interchain_counter":        meta.InterchainCounter,
						"receipt_counter":           meta.ReceiptCounter,
						"source_interchain_counter": meta.SourceInterchainCounter,
						"source_receipt_counter":    meta.SourceReceiptCounter,
					}).Infof("Pier information of service %s", serviceID)
				}
				if err := pier.exchanger.Start(); err != nil {
					pier.logger.Errorf("exchanger start: %w", err)
					return
				}
				status = true
			} else {
				if !status {
					continue
				}
				if err := pier.Stop(true); err != nil {
					pier.logger.Errorf("pier stop: %w", err)
					return
				}
				status = false
			}
		}
	}
}

// Stop stops three main components of pier app
func (pier *Pier) Stop(isAux bool) error {
	if pier.config.Mode.Type != repo.UnionMode {
		if err := pier.monitor.Stop(); err != nil {
			return fmt.Errorf("monitor stop: %w", err)
		}

		if !isAux {
			// stop appchain plugin first and kill plugin process
			pier.grpcPlugin.Kill()
		}

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

func filterServiceMeta(serviceInterchain map[string]*pb.Interchain, bxhID, appchainID string, serviceIDs []string) map[string]*pb.Interchain {
	result := make(map[string]*pb.Interchain)

	for _, id := range serviceIDs {
		fullServiceID := fmt.Sprintf("%s:%s:%s", bxhID, appchainID, id)
		val, ok := serviceInterchain[fullServiceID]
		if !ok {
			val = &pb.Interchain{
				ID:                      fullServiceID,
				InterchainCounter:       make(map[string]uint64),
				ReceiptCounter:          make(map[string]uint64),
				SourceInterchainCounter: make(map[string]uint64),
				SourceReceiptCounter:    make(map[string]uint64),
			}
		}
		result[fullServiceID] = val
	}

	return result
}

func newBitXHubClient(logger logrus.FieldLogger, privateKey crypto.PrivateKey, config *repo.Config) (rpcx.Client, error) {
	opts := []rpcx.Option{
		rpcx.WithLogger(logger),
		rpcx.WithPrivateKey(privateKey),
	}
	nodesInfo := make([]*rpcx.NodeInfo, 0, len(config.Mode.Relay.Addrs))
	for index, addr := range config.Mode.Relay.Addrs {
		nodeInfo := &rpcx.NodeInfo{Addr: addr}
		if config.Security.EnableTLS {
			nodeInfo.CertPath = filepath.Join(config.RepoRoot, config.Security.Tlsca)
			nodeInfo.EnableTLS = config.Security.EnableTLS
			nodeInfo.CommonName = config.Security.CommonName
			nodeInfo.AccessCert = filepath.Join(config.RepoRoot, config.Security.AccessCert[index])
			nodeInfo.AccessKey = filepath.Join(config.RepoRoot, config.Security.AccessKey)
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	opts = append(opts, rpcx.WithNodesInfo(nodesInfo...), rpcx.WithTimeoutLimit(config.Mode.Relay.TimeoutLimit))
	return rpcx.New(opts...)
}
