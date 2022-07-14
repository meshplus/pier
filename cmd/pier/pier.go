package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier"
	"github.com/urfave/cli"
)

var logger = log.NewWithModule("cmd")

func main() {
	app := cli.NewApp()
	app.Name = "Pier"
	app.Usage = "A Gateway Used To Cross the Appchain"
	app.Compiled = time.Now()
	app.Version = fmt.Sprintf("Pier version: %s-%s-%s\n", pier.CurrentVersion, pier.CurrentBranch, pier.CurrentCommit)

	// global flags
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "repo",
			Usage: "Pier repository path",
		},
		cli.BoolFlag{
			Name:  "tls",
			Usage: "enable tls between pier and bitxhub or not",
		},
	}

	app.Commands = []cli.Command{
		appchainBxhCMD,
		keyCMD,
		idCMD,
		initCMD,
		networkCMD,
		p2pCMD,
		pluginCMD,
		startCMD,
		versionCMD,
	}

	err := app.Run(os.Args)
	if err != nil {
		color.Red(err.Error())
		os.Exit(-1)
	}
}
