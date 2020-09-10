package main

import (
	"fmt"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var idCMD = cli.Command{
	Name:  "id",
	Usage: "Get appchain id",
	Action: func(ctx *cli.Context) error {
		repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
		if err != nil {
			return err
		}

		keyPath := filepath.Join(repoRoot, "key.json")

		privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
		if err != nil {
			return err
		}

		address, err := privKey.PublicKey().Address()
		if err != nil {
			return err
		}

		fmt.Println(address.String())

		return nil
	},
}
