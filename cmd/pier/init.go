package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/meshplus/pier/internal/repo"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

var initCMD = cli.Command{
	Name:  "init",
	Usage: "Initialize pier core local configuration",
	Flags: []cli.Flag{
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
			Value:    "certs/ca.pem",
		},
		cli.StringFlag{
			Name:     "common-name",
			Usage:    "Specify common name to verify",
			Required: false,
			Value:    "localhost",
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
			Name:  "relay",
			Usage: "Initialize pier relay mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addrs",
					Usage:    "Specify bitxhub nodes' address",
					Required: false,
					Value:    &cli.StringSlice{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				},
				cli.Uint64Flag{
					Name:     "quorum",
					Usage:    "Specify the quorum number of BitXHub",
					Required: false,
					Value:    2,
				},
				cli.StringSliceFlag{
					Name:     "validators",
					Usage:    "Specify validators of bitxhub",
					Required: false,
					Value: &cli.StringSlice{
						"0x000f1a7a08ccc48e5d30f80850cf1cf283aa3abd",
						"0xe93b92f1da08f925bdee44e91e7768380ae83307",
						"0xb18c8575e3284e79b92100025a31378feb8100d6",
						"0x856E2B9A5FA82FD1B031D1FF6863864DBAC7995D",
					},
				},
			},
			Action: initPier,
		},
		{
			Name:  "direct",
			Usage: "Initialize pier direct mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "peers",
					Usage:    "Specify counter party peers to connect",
					Required: true,
				},
			},
			Action: initPier,
		},
		{
			Name:  "union",
			Usage: "Initialize pier union mode configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addrs",
					Usage:    "Specify bitxhub nodes' address",
					Required: false,
					Value:    &cli.StringSlice{"localhost:60011", "localhost:60012", "localhost:60013", "localhost:60014"},
				},
				cli.StringSliceFlag{
					Name:     "connectors",
					Usage:    "Specify the remote union peers to connect",
					Required: true,
				},
			},
			Action: initPier,
		},
	},
}

func initRepo(repoRoot string) error {
	if fileutil.Exist(filepath.Join(repoRoot, repo.ConfigName)) {
		fmt.Println("pier configuration file already exists")
		fmt.Println("reinitializing would overwrite your configuration, Y/N?")
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
		if input.Text() == "Y" || input.Text() == "y" {
			return repo.Initialize(repoRoot)
		}

		return nil
	}

	return repo.Initialize(repoRoot)
}

func initPier(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	if err := initRepo(repoRoot); err != nil {
		return err
	}

	vpr, err := readFromConfigFile(filepath.Join(repoRoot, repo.ConfigName))
	if err != nil {
		return err
	}

	mode := ctx.Command.Name
	switch mode {
	case "relay":
		addrs := ctx.StringSlice("addrs")
		quorum := ctx.Uint64("quorum")
		validators := ctx.StringSlice("validators")
		vpr.Set("mode.type", "relay")
		vpr.Set("mode.relay.addrs", addrs)
		vpr.Set("mode.relay.quorum", quorum)
		vpr.Set("mode.relay.validators", validators)
	case "direct":
		peers := ctx.StringSlice("peers")
		vpr.Set("mode.type", "direct")
		vpr.Set("mode.direct.peers", peers)
	case "union":
		addrs := ctx.StringSlice("addrs")
		connectors := ctx.StringSlice("connectors")
		vpr.Set("mode.type", "union")
		vpr.Set("mode.union.addrs", addrs)
		vpr.Set("mode.union.connectors", connectors)
	}

	if err := updateInitOptions(ctx, vpr, repoRoot); err != nil {
		return err
	}

	return vpr.WriteConfig()
}

func updateInitOptions(ctx *cli.Context, vpr *viper.Viper, repoRoot string) error {
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

	keyPath := filepath.Join(repoRoot, repo.KeyName)
	privKey, err := asym.RestorePrivateKey(keyPath, repo.KeyPassword)
	if err != nil {
		return err
	}

	addr, err := privKey.PublicKey().Address()
	if err != nil {
		return err
	}

	vpr.Set("appchain.did", fmt.Sprintf("did:bitxhub:appchain%s:.", addr.String()))

	return nil
}

func readFromConfigFile(cfgFile string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}
