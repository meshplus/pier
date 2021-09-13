package main

import "github.com/urfave/cli"

var networkCMD = cli.Command{
	Name:  "network",
	Usage: "Modify pier network configuration",
	Subcommands: []cli.Command{
		{
			Name:  "relay",
			Usage: "Modify pier relay mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addNode",
					Usage:    "add bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delNode",
					Usage:    "delete bitxhub node address",
					Required: false,
				},
			},
			Action: configNetwork,
		},
		{
			Name:  "direct",
			Usage: "Modify pier direct mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "add pier address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delPier",
					Usage:    "delete pier address",
					Required: false,
				},
			},
			Action: configNetwork,
		},
		{
			Name:  "union",
			Usage: "Modify pier union mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addNode",
					Usage:    "add bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delNode",
					Usage:    "delete bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "add pier address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delPier",
					Usage:    "delete pier address",
					Required: false,
				},
			},
			Action: configNetwork,
		},
	},
}

func configNetwork(ctx *cli.Context) error {

	return nil

}
