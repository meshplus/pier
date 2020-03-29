package main

import (
	"fmt"
	"io/ioutil"

	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/tidwall/gjson"
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

	client, err := loadClient(repo.KeyPath(repoRoot), config.Bitxhub.Addr)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	contract, err := ioutil.ReadFile(rulePath)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(repo.KeyPath(repoRoot))
	if err != nil {
		return err
	}

	address := gjson.Get(string(data), "address")

	contractAddr, err := client.DeployContract(contract)
	if err != nil {
		return fmt.Errorf("deploy rule: %w", err)
	}

	_, err = client.InvokeBVMContract(
		rpcx.RuleManagerContractAddr,
		"RegisterRule",
		rpcx.String(address.String()),
		rpcx.String(contractAddr.String()))
	if err != nil {
		return fmt.Errorf("register rule")
	}

	fmt.Println("Deploy rule to bitxhub successfully")

	return nil
}
