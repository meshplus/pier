package main

import (
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-kit/log"
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
	}

	app.Commands = []cli.Command{
		appchainCMD,
		idCMD,
		initCMD,
		interchainCMD,
		ruleCMD,
		startCMD,
		versionCMD,
	}

	err := app.Run(os.Args)
	if err != nil {
		color.Red(err.Error())
		os.Exit(-1)
	}
}
