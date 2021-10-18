package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-core/agency"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	_ "github.com/meshplus/pier/imports"
	"github.com/meshplus/pier/internal/adapt/appchain_adapter"
	"github.com/meshplus/pier/internal/adapt/bxh_adapter"
	"github.com/meshplus/pier/internal/adapt/direct_adapter"
	"github.com/meshplus/pier/internal/adapt/union_adapter"
	exchanger2 "github.com/meshplus/pier/internal/exchanger2"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	_ "github.com/meshplus/pier/pkg/single"
	"github.com/sirupsen/logrus"
	"github.com/wonderivan/logger"
)

const DEFAULT_UNION_PIER_ID = "default_union_pier_id"

// Pier represents the necessary data for starting the pier app
type Pier struct {
	privateKey  crypto.PrivateKey
	plugin      plugins.Client
	grpcPlugin  *plugin.Client
	pierHA      agency.PierHA
	storage     storage.Storage
	exchanger   exchanger2.IExchanger
	ctx         context.Context
	cancel      context.CancelFunc
	appchain    *appchainmgr.Appchain
	serviceMeta map[string]*pb.Interchain
	config      *repo.Config
	logger      logrus.FieldLogger
}

func NewUnionPier(repoRoot string, config *repo.Config) (*Pier, error) {
	logger := loggers.Logger(loggers.App)
	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	client, err := newBitXHubClient(logger, privateKey, config)
	if err != nil {
		return nil, fmt.Errorf("create bitxhub client: %w", err)
	}

	bxhAdapter, err := bxh_adapter.New(repo.UnionMode, DEFAULT_UNION_PIER_ID, client, loggers.Logger(loggers.Syncer))
	if err != nil {
		return nil, fmt.Errorf("new bitxhub adapter: %w", err)
	}

	nodePrivKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load node key: %w", err)
	}

	peerManager, err := peermgr.New(config, nodePrivKey, privateKey, config.Mode.Union.Providers, loggers.Logger(loggers.PeerMgr))
	if err != nil {
		return nil, fmt.Errorf("peerMgr create: %w", err)
	}

	unionAdapt, err := union_adapter.New(peerManager, bxhAdapter, loggers.Logger(loggers.Union))
	if err != nil {
		return nil, fmt.Errorf("new union adapter: %w", err)
	}

	ex, err := exchanger2.New(repo.UnionMode, "", bxhAdapter.ID(),
		exchanger2.WithSrcAdapt(bxhAdapter),
		exchanger2.WithDestAdapt(unionAdapt),
		exchanger2.WithLogger(loggers.Logger(loggers.Exchanger)))
	if err != nil {
		return nil, fmt.Errorf("exchanger create: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Pier{
		privateKey: privateKey,
		exchanger:  ex,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
	}, nil
}

// Start starts three main components of pier app
func (pier *Pier) Start() error {
	if pier.config.Mode.Type == repo.UnionMode {
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
		ex          exchanger2.IExchanger
		pierHA      agency.PierHA
		peerManager peermgr.PeerManager
	)

	logger := loggers.Logger(loggers.App)
	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load key: %w", err)
	}

	nodePrivKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("repo load node key: %w", err)
	}

	switch config.Mode.Type {
	case repo.DirectMode:
		peerManager, err = peermgr.New(config, nodePrivKey, privateKey, 1, loggers.Logger(loggers.PeerMgr))
		if err != nil {
			return nil, fmt.Errorf("peerMgr create: %w", err)
		}
		cryptor, err := txcrypto.NewDirectCryptor(peerManager, privateKey)
		appchainAdapter, err := appchain_adapter.NewAppchainAdapter(config, loggers.Logger(loggers.Appchain), cryptor)
		if err != nil {
			return nil, fmt.Errorf("new appchain adapter: %w", err)
		}

		directAdapter, err := direct_adapter.New(peerManager, appchainAdapter, loggers.Logger(loggers.Direct))
		if err != nil {
			return nil, fmt.Errorf("new direct adapter: %w", err)
		}

		ex, err = exchanger2.New(repo.DirectMode, config.Appchain.ID, "",
			exchanger2.WithSrcAdapt(appchainAdapter),
			exchanger2.WithDestAdapt(directAdapter),
			exchanger2.WithLogger(log.NewWithModule("exchanger")))
		if err != nil {
			return nil, fmt.Errorf("exchanger create: %w", err)
		}

		return nil, fmt.Errorf("direct mode is unsupported yet")
	case repo.RelayMode:
		client, err := newBitXHubClient(logger, privateKey, config)
		if err != nil {
			return nil, fmt.Errorf("create bitxhub client: %w", err)
		}

		cryptor, err := txcrypto.NewRelayCryptor(client, privateKey)
		appchainAdapter, err := appchain_adapter.NewAppchainAdapter(config, loggers.Logger(loggers.Appchain), cryptor)
		if err != nil {
			return nil, fmt.Errorf("new appchain adapter: %w", err)
		}

		bxhAdapter, err := bxh_adapter.New(repo.RelayMode, appchainAdapter.ID(), client, loggers.Logger(loggers.Syncer))

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
		if !isAux {
			// stop appchain plugin first and kill plugin process
			pier.grpcPlugin.Kill()
		}
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
