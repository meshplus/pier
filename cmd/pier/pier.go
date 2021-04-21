package main

import (
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/cmd/pier/client"
	"github.com/urfave/cli"
)

var logger = log.NewWithModule("cmd")

func main() {
	app := cli.NewApp()
	app.Name = "Pier"
	app.Usage = "Manipulate the crosschain node"
	app.Compiled = time.Now()

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
		client.LoadClientCMD(),
		idCMD,
		initCMD,
		interchainCMD,
		p2pCMD,
		ruleCMD,
		startCMD,
		versionCMD,
		governanceCMD,
	}

	err := app.Run(os.Args)
	if err != nil {
		color.Red(err.Error())
		os.Exit(-1)
	}
}
