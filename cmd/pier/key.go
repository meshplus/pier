package main

import (
	"fmt"

	"github.com/meshplus/bitxhub-kit/hexutil"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var keyCMD = cli.Command{
	Name:   "show",
	Usage:  "Show pier key from repo",
	Action: showKey,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "path",
			Usage:    "Specify private key path",
			Required: true,
		},
		cli.StringFlag{
			Name:     "passwd",
			Usage:    "Specify password",
			Required: false,
		},
	},
}

func showKey(ctx *cli.Context) error {
	privPath := ctx.String("path")
	passwd := ctx.String("passwd")
	if passwd == "" {
		passwd = repo.KeyPassword
	}

	privKey, err := asym.RestorePrivateKey(privPath, passwd)
	if err != nil {
		return err
	}

	data, err := privKey.Bytes()
	if err != nil {
		return err
	}

	pubData, err := privKey.PublicKey().Bytes()
	if err != nil {
		return err
	}
	addr, err := privKey.PublicKey().Address()
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("private key: %s", hexutil.Encode(data)))
	fmt.Println(fmt.Sprintf("public key: %s", hexutil.Encode(pubData)))
	fmt.Println(fmt.Sprintf("address: %s", addr))

	return nil
}
