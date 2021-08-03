package main

import (
	"encoding/json"
	"fmt"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
	"path"
)

var serviceCommand = cli.Command{
	Name:  "service",
	Usage: "Command about appchain service",
	Subcommands: []cli.Command{
		{
			Name:  "register",
			Usage: "Register appchain service info to bitxhub",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "appchain-id",
					Usage:    "Specify appchain ID",
					Required: true,
				},
				cli.StringFlag{
					Name:     "service-id",
					Usage:    "Specify service ID",
					Required: true,
				},
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specify service name",
					Required: true,
				},
				cli.StringFlag{
					Name:     "desc",
					Usage:    "Specify service description",
					Required: true,
				},
				cli.StringFlag{
					Name:     "type",
					Usage:    "Specify service type",
					Required: true,
				},
				cli.BoolFlag{
					Name:     "ordered",
					Usage:    "Specify if the service should be ordered",
					Required: true,
				},
				cli.StringFlag{
					Name:     "permit",
					Usage:    "Specify service permission",
					Required: true,
				},
				cli.StringFlag{
					Name:     "items",
					Usage:    "Specify service items",
					Required: true,
				},
			},
			Action: registerService,
		},
	},
}

func registerService(ctx *cli.Context) error {
	appchainID := ctx.String("appchain-id")
	serviceID := ctx.String("service-id")
	name := ctx.String("name")
	desc := ctx.String("desc")
	typ := ctx.String("type")
	ordered := ctx.Bool("ordered")
	permit := ctx.String("permit")
	items := ctx.String("items")

	repoRoot, err := repo.PathRoot()
	if err != nil {
		return err
	}

	client, _, err := initClientWithKeyPath(ctx, path.Join(repoRoot, repo.KeyName))
	if err != nil {
		return err
	}
	// init method registry with this admin key
	receipt, err := client.InvokeBVMContract(
		constant.ServiceMgrContractAddr.Address(),
		"Register", nil,
		rpcx.String(appchainID),
		rpcx.String(serviceID),
		rpcx.String(name),
		rpcx.String(desc),
		rpcx.String(typ),
		rpcx.Bool(ordered),
		rpcx.String(permit),
		rpcx.Bytes([]byte(items)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("register service info faild: %s", string(receipt.Ret))
	}
	ret := &GovernanceResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}
	fmt.Printf("Register appchain service info for %s successfully, wait for proposal %s to finish.\n", string(ret.Extra), ret.ProposalID)
	return nil
}
