package main

import (
	"fmt"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/key"
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
		key, err := key.LoadKey(keyPath)

		fmt.Println(key.Address)

		return nil
	},
}
