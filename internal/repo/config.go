package repo

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/spf13/viper"
)

type Repo struct {
	Config        *Config
	NetworkConfig *NetworkConfig
}

// Config represents the necessary config data for starting pier
type Config struct {
	RepoRoot string
	Title    string   `toml:"title" json:"title"`
	Port     Port     `toml:"port" json:"port"`
	Mode     Mode     `toml:"mode" json:"mode"`
	Log      Log      `toml:"log" json:"log"`
	Appchain Appchain `toml:"appchain" json:"appchain"`
	Security Security `toml:"security" json:"security"`
	HA       HA       `toml:"ha" json:"ha"`
	TSS      *TSS     `toml:"tss" json:"tss"`
	Batch    Batch    `toml:"batch" json:"batch"`
}

// Security are certs used to setup connection with tls
type Security struct {
	EnableTLS  bool     `mapstructure:"enable_tls"`
	AccessCert []string `mapstructure:"access_cert"`
	AccessKey  string   `mapstructure:"access_key"`
	Tlsca      string   `toml:"tlsca" json:"tlsca"`
	CommonName string   `mapstructure:"common_name" json:"common_name"`
}

// Port are ports providing http and pprof service
type Port struct {
	Http  int64 `toml:"http" json:"http"`
	PProf int64 `toml:"pprof" json:"pprof"`
}

type HA struct {
	Mode string `toml:"mode" json:"mode"`
}

type TSS struct {
	EnableTSS bool `mapstructure:"enable_tss" json:"enable_tss"`
}

type Batch struct {
	EnableBatch bool `mapstructure:"enable_batch" json:"enable_batch"`
	BatchSize   int  `mapstructure:"batch_size" json:"batch_size"`
}

const (
	DirectMode = "direct"
	RelayMode  = "relay"
	UnionMode  = "union"
)

type Mode struct {
	Type   string `toml:"type" json:"type"`
	Relay  Relay  `toml:"relay" json:"relay"`
	Direct Direct `toml:"direct" json:"direct"`
	Union  Union  `toml:"union" json:"union"`
}

// Relay are configs about bitxhub
type Relay struct {
	Addrs                      []string      `toml:"addrs" json:"addrs"`
	TimeoutLimit               time.Duration `mapstructure:"timeout_limit" json:"timeout_limit"`
	Quorum                     uint64        `toml:"quorum" json:"quorum"`
	BitXHubID                  string        `mapstructure:"bitxhub_id" json:"bitxhub_id"`
	EnableOffChainTransmission bool          `mapstructure:"enable_offchain_transmission" json:"enable_offchain_transmission"`
	GrpcPoolSize               int           `mapstructure:"grpc_pool_size" json:"grpc_pool_size"`
	SavePath                   string        `mapstructure:"save_path" json:"save_path"`
}

type Direct struct {
	GasLimit uint64 `toml:"gas_limit" json:"gas_limit"`
}

type Union struct {
	Addrs     []string `toml:"addrs" json:"addrs"`
	Providers uint64   `toml:"providers" json:"providers"`
}

// Log are config about log
type Log struct {
	Dir          string    `toml:"dir" json:"dir"`
	Filename     string    `toml:"filename" json:"filename"`
	ReportCaller bool      `mapstructure:"report_caller"`
	Level        string    `toml:"level" json:"level"`
	Module       LogModule `toml:"module" json:"module"`
}

type LogModule struct {
	ApiServer   string `mapstructure:"api_server" toml:"api_server" json:"api_server"`
	AppchainMgr string `mapstructure:"appchain_mgr" toml:"appchain_mgr" json:"appchain_mgr"`
	BxhLite     string `mapstructure:"bxh_lite" toml:"bxh_lite" json:"bxh_lite"`
	Exchanger   string `toml:"exchanger" json:"exchanger"`
	Executor    string `toml:"executor" json:"executor"`
	Monitor     string `toml:"monitor" json:"monitor"`
	PeerMgr     string `mapstructure:"peer_mgr" toml:"peer_mgr" json:"peer_mgr"`
	Router      string `toml:"router" json:"router"`
	RuleMgr     string `mapstructure:"rule_mgr" toml:"rule_mgr" json:"rule_mgr"`
	Swarm       string `toml:"swarm" json:"swarm"`
	Appchain    string `mapstructure:"appchain_adapter" toml:"appchain_adapter" json:"appchain_adapter"`
	Syncer      string `mapstructure:"bxh_adapter" toml:"bxh_adapter" json:"bxh_adapter"`
	Direct      string `mapstructure:"direct_adapter" toml:"direct_adapter" json:"direct_adapter"`
	Union       string `toml:"union_adapter" json:"union_adapter"`
	Cryptor     string `toml:"cryptor" json:"cryptor"`
}

// Appchain are configs about appchain
type Appchain struct {
	ID     string `toml:"id" json:"id"`
	Config string `toml:"config" json:"config"`
	Plugin string `toml:"plugin" json:"plugin"`
}

// DefaultConfig returns config with default value
func DefaultConfig() *Config {
	return &Config{
		RepoRoot: ".pier",
		Title:    "pier configuration file",
		Port: Port{
			Http:  8080,
			PProf: 44555,
		},
		Mode: Mode{
			Type: "relay",
			Relay: Relay{
				//Addrs:                      []string{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				Quorum:                     2,
				BitXHubID:                  "1356",
				EnableOffChainTransmission: false,
				GrpcPoolSize:               16,
			},
			Direct: Direct{
				GasLimit: 0x5f5e100,
			},
			Union: Union{
				//Addrs:     []string{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				Providers: 1,
			},
		},
		Log: Log{
			Dir:          "logs",
			Filename:     "pier.log",
			ReportCaller: false,
			Level:        "info",
			Module: LogModule{
				AppchainMgr: "info",
				Exchanger:   "info",
				Executor:    "info",
				BxhLite:     "info",
				Monitor:     "info",
				Swarm:       "info",
				RuleMgr:     "info",
				Syncer:      "info",
				PeerMgr:     "info",
				Router:      "info",
				ApiServer:   "info",
				Direct:      "info",
				Union:       "info",
				Appchain:    "info",
			},
		},
		Security: Security{
			EnableTLS: false,
			Tlsca:     "certs/agency.cert",
			//AccessCert: []string{"node1.cert", "node2.cert", "node3.cert", "node4.cert"},
			AccessKey:  "node.priv",
			CommonName: "localhost",
		},
		HA: HA{
			Mode: "single",
		},
		Appchain: Appchain{
			ID:     "appchain",
			Plugin: "appchain_plugin",
			Config: "fabric",
		},
		TSS: &TSS{
			EnableTSS: false,
		},
		Batch: Batch{
			EnableBatch: false,
			BatchSize:   10,
		},
	}
}

// UnmarshalConfig read from config files under config path
func UnmarshalConfig(repoRoot string) (*Config, error) {
	configPath := filepath.Join(repoRoot, ConfigName)

	if !fileutil.Exist(configPath) {
		return nil, fmt.Errorf("file %s doesn't exist, please initialize pier firstly", configPath)
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("PIER")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	config := DefaultConfig()

	if err := viper.Unmarshal(config); err != nil {
		return nil, err
	}

	if len(config.Mode.Relay.Addrs) == 0 {
		config.Mode.Relay.Addrs = []string{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"}
	}

	if len(config.Mode.Union.Addrs) == 0 {
		config.Mode.Union.Addrs = []string{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"}
	}

	if len(config.Security.AccessCert) == 0 {
		config.Security.AccessCert = []string{"node1.cert", "node2.cert", "node3.cert", "node4.cert"}
	}

	config.RepoRoot = repoRoot

	return config, nil
}
