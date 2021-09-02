package main

import (
	"fmt"
	"path/filepath"

	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var governanceCMD = cli.Command{
	Name:  "proposals",
	Usage: "proposals manage command",
	Subcommands: cli.Commands{
		cli.Command{
			Name:  "withdraw",
			Usage: "withdraw a proposal",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "id",
					Usage:    "proposal id",
					Required: true,
				},
				governanceReasonFlag,
			},
			Action: withdraw,
		},
	},
}

func withdraw(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	id := ctx.String("id")
	reason := ctx.String("reason")

	keyPath := filepath.Join(repoRoot, "key.json")
	client, _, err := initClientWithKeyPath(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.GovernanceContractAddr.Address(),
		"WithdrawProposal", nil, rpcx.String(id), rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke withdraw proposal: %s", receipt.Ret)
	}

	return nil
}
