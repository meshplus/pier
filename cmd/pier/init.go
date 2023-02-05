package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/meshplus/bitxhub-kit/types"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

const (
	relayStr  = "relay"
	directStr = "direct"
	unionStr  = "union"
)

var initCMD = cli.Command{
	Name:  "init",
	Usage: "Initialize pier local configuration",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "algo",
			Usage:    "Specify crypto algorithm",
			Value:    "Secp256k1",
			Required: false,
		},
		cli.Uint64Flag{
			Name:     "http-port",
			Usage:    "Specify http port",
			Required: false,
			Value:    44544,
		},
		cli.Uint64Flag{
			Name:     "pprof-port",
			Usage:    "Specify pprof port",
			Required: false,
			Value:    44555,
		},
		cli.BoolFlag{
			Name:     "enable-tls",
			Usage:    "Enable TLS or not",
			Required: false,
		},
		cli.StringFlag{
			Name:     "tlsca",
			Usage:    "Specify TLS CA certificate path",
			Required: false,
			Value:    "certs/agency.cert",
		},
		cli.StringFlag{
			Name:     "common-name",
			Usage:    "Specify common name to verify",
			Required: false,
			Value:    "BitXHub",
		},
		cli.StringFlag{
			Name:     "ha",
			Usage:    "Specify if pier will run in single mode or high availability mode",
			Required: false,
			Value:    "single",
		},
	},
	Subcommands: []cli.Command{
		{
			Name:  relayStr,
			Usage: "Initialize pier relay mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addrs",
					Usage:    "Specify bitxhub nodes' address",
					Required: false,
					Value:    &cli.StringSlice{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				},
				cli.StringFlag{
					Name:     "quorum",
					Usage:    "Specify the quorum number of BitXHub",
					Required: false,
					Value:    "2",
				},
			},
			Action: initPier,
		},
		{
			Name:  directStr,
			Usage: "Initialize pier direct mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "Specify counter party piers to connect, input looks like: [ip]:[port]#[Pid]",
					Required: false,
				},
			},
			Action: initPier,
		},
		{
			Name:  unionStr,
			Usage: "Initialize pier union mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addrs",
					Usage:    "Specify bitxhub nodes' address",
					Required: false,
					Value:    &cli.StringSlice{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				},
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "Specify the remote union piers to connect, input looks like: [ip]:[port]#[Pid]",
					Required: false,
				},
			},
			Action: initPier,
		},
	},
}

func initRepo(repoRoot string, algo string) error {
	if fileutil.Exist(filepath.Join(repoRoot, repo.ConfigName)) {
		fmt.Println("pier configuration file already exists")
		fmt.Println("reinitializing would overwrite your configuration, Y/N?")
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
		if input.Text() == "Y" || input.Text() == "y" {
			return repo.Initialize(repoRoot, algo)
		}

		return nil
	}

	return repo.Initialize(repoRoot, algo)
}

func initPier(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	algo := ctx.GlobalString("algo")
	if err := initRepo(repoRoot, algo); err != nil {
		return err
	}

	vpr, err := ReadFromConfigFile(filepath.Join(repoRoot, repo.ConfigName))
	if err != nil {
		return err
	}

	mode := ctx.Command.Name
	switch mode {
	case relayStr:
		vpr.Set("mode.type", relayStr)

		addrs := ctx.StringSlice("addrs")
		defaltaddrs := repo.DefaultConfig().Mode.Relay.Addrs
		// if user input addr on Command, remove default addr
		if len(addrs) > len(defaltaddrs) {
			addrs = addrs[len(defaltaddrs):]
		}
		vpr.Set("mode.relay.addrs", addrs)

		quorum := ctx.String("quorum")
		in, err := strconv.Atoi(quorum)
		if err != nil {
			return fmt.Errorf("quorum type err: %w", err)
		}
		vpr.Set("mode.relay.quorum", in)
	case directStr:
		if err := updateNetworkAddrs(ctx, nil, repoRoot, mode); err != nil {
			return err
		}
		vpr.Set("mode.type", directStr)

	case unionStr:
		addrs := ctx.StringSlice("addrs")
		defaltaddrs := repo.DefaultConfig().Mode.Union.Addrs
		// if user input addr on Command, remove default addr
		if len(addrs) > len(defaltaddrs) {
			addrs = addrs[len(defaltaddrs):]
		}
		vpr.Set("mode.union.addrs", addrs)

		if err := updateNetworkAddrs(ctx, nil, repoRoot, mode); err != nil {
			return err
		}
		vpr.Set("mode.type", unionStr)

	}

	if err := updateInitOptions(ctx, vpr, repoRoot); err != nil {
		return err
	}

	return vpr.WriteConfig()
}

func updateInitOptions(ctx *cli.Context, vpr *viper.Viper, repoRoot string) error {
	_ = repoRoot
	httpPort := ctx.GlobalUint64("http-port")
	pprofPort := ctx.GlobalUint64("pprof-port")
	enableTls := ctx.GlobalBool("enable-tls")
	tlsca := ctx.GlobalString("tlsca")
	commonName := ctx.GlobalString("common-name")
	ha := ctx.GlobalString("ha")

	vpr.Set("port.http", httpPort)
	vpr.Set("port.pprof", pprofPort)
	vpr.Set("security.enable_tls", enableTls)
	vpr.Set("security.tlsca", tlsca)
	vpr.Set("security.common_name", commonName)
	vpr.Set("HA.mode", ha)

	return nil
}

func ReadFromConfigFile(cfgFile string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}

func initClientWithKeyPath(ctx *cli.Context, chainAdminKeyPath string) (rpcx.Client, *types.Address, error) {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return nil, nil, err
	}

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("init config error: %s", err)
	}

	adminPriv, err := asym.RestorePrivateKey(chainAdminKeyPath, "bitxhub")
	if err != nil {
		return nil, nil, err
	}
	address, err := adminPriv.PublicKey().Address()
	if err != nil {
		return nil, nil, err
	}

	addrs := []string{}
	switch config.Mode.Type {
	case repo.RelayMode:
		addrs = config.Mode.Relay.Addrs
	case repo.DirectMode:
		//TODO: Direct model doesn't need this function, Not sure the process is correct.
		break
	case repo.UnionMode:
		addrs = config.Mode.Union.Addrs
	}

	client, err := loadClient(chainAdminKeyPath, addrs, ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load client: %w", err)
	}
	return client, address, nil
}
