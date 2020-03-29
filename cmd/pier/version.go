package main

import (
	"fmt"

	"github.com/meshplus/pier"
	"github.com/urfave/cli"
)

var versionCMD = cli.Command{
	Name:  "version",
	Usage: "Show version about ap",
	Action: func(ctx *cli.Context) error {
		fmt.Print(getVersion(true))

		return nil
	},
}

func getVersion(all bool) string {
	version := fmt.Sprintf("Pier version: %s-%s\n", pier.CurrentVersion, pier.CurrentCommit)
	if all {
		version += fmt.Sprintf("App build date: %s\n", pier.BuildDate)
		version += fmt.Sprintf("System version: %s\n", pier.Platform)
		version += fmt.Sprintf("Golang version: %s\n", pier.GoVersion)
	}

	return version
}
