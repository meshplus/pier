package main

import (
	"fmt"
	"io/ioutil"

	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var ruleCMD = cli.Command{
	Name:  "rule",
	Usage: "Command about rule",
	Subcommands: cli.Commands{
		{
			Name:  "deploy",
			Usage: "Deploy validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "path",
					Usage:    "Specific rule path",
					Required: true,
				},
			},
			Action: deployRule,
		},
	},
}

func deployRule(ctx *cli.Context) error {
	rulePath := ctx.String("path")

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("init config error: %s", err)
	}

	client, err := loadClient(repo.KeyPath(repoRoot), config.Mode.Relay.Addr, ctx)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	contract, err := ioutil.ReadFile(rulePath)
	if err != nil {
		return err
	}

	privateKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return fmt.Errorf("repo load key: %w", err)
	}

	address, err := privateKey.PublicKey().Address()
	if err != nil {
		return fmt.Errorf("get address from private key %w", err)
	}

	contractAddr, err := client.DeployContract(contract, nil)
	if err != nil {
		return fmt.Errorf("deploy rule: %w", err)
	}

	_, err = client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"RegisterRule", nil,
		rpcx.String(address.String()),
		rpcx.String(contractAddr.String()))
	if err != nil {
		return fmt.Errorf("register rule")
	}

	fmt.Println("Deploy rule to bitxhub successfully")

	return nil
}
