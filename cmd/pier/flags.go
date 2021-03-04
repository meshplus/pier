package main

import "github.com/urfave/cli"

var (
	bxhAddrFlag = cli.StringFlag{
		Name:     "addr",
		Usage:    "Specific bitxhub node address",
		Value:    "localhost:60011",
		Required: false,
	}
	adminKeyPathFlag = cli.StringFlag{
		Name:     "admin-key",
		Usage:    "Specific admin key path",
		Required: true,
	}
	methodFlag = cli.StringFlag{
		Name:     "method",
		Usage:    "Specific did sub method name(like appchain)",
		Required: true,
	}
	didFlag = cli.StringFlag{
		Name:     "did",
		Usage:    "Specific full did name(like did:bitxhub:appchain1:0xc7F999b83Af6DF9e67d0a37Ee7e900bF38b3D013)",
		Required: true,
	}
	statusFlag = cli.IntFlag{
		Name:     "status",
		Usage:    "Specify the status you want to set(1 is pass, 0 is reject, default is 1)",
		Required: false,
		Value:    1,
	}
	didDocAddrFlag = cli.StringFlag{
		Name:     "doc-addr",
		Usage:    "Specify the ipfs addr of did document",
		Required: true,
	}
	didDocHashFlag = cli.StringFlag{
		Name:     "doc-hash",
		Usage:    "Specify the hash of did document",
		Required: true,
	}

	// appchain info related flags
	appchainNameFlag = cli.StringFlag{
		Name:     "name",
		Usage:    "Specific appchain name",
		Required: true,
	}
	appchainTypeFlag = cli.StringFlag{
		Name:     "type",
		Usage:    "Specific appchain type",
		Required: true,
	}
	appchainDescFlag = cli.StringFlag{
		Name:     "desc",
		Usage:    "Specific appchain description",
		Required: true,
	}
	appchainVersionFlag = cli.StringFlag{
		Name:     "version",
		Usage:    "Specific appchain version",
		Required: true,
	}
	appchainValidatorFlag = cli.StringFlag{
		Name:     "validators",
		Usage:    "Specific appchain validators path",
		Required: true,
	}
)
