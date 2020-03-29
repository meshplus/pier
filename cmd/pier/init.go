package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var initCMD = cli.Command{
	Name:  "init",
	Usage: "Initialize pier local configuration",
	Action: func(ctx *cli.Context) error {
		repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
		if err != nil {
			return err
		}

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
	},
}
