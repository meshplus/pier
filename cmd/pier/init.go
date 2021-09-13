package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/meshplus/bitxhub-kit/types"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
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
	},
	Action: func(ctx *cli.Context) error {
		repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
		if err != nil {
			return err
		}

		algo := ctx.String("algo")

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
	},
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
