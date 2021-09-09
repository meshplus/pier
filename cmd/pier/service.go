package main

import (
	"encoding/json"
	"fmt"
	"path"

	service_mgr "github.com/meshplus/bitxhub-core/service-mgr"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
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
					Usage:    "Specify service ID（address of Service Contract）",
					Required: true,
				},
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specify service name",
					Required: true,
				},
				cli.StringFlag{
					Name:     "intro",
					Usage:    "Specify service description",
					Required: true,
				},
				cli.StringFlag{
					Name:     "type",
					Usage:    "Specify service type, one of \"CallContract\", \"DepositCertificate\", \"DataMigration\", current only support CallContract",
					Value:    "CallContract",
					Required: false,
				},
				cli.BoolFlag{
					Name:  "ordered",
					Usage: "Specify if the service should be ordered",
				},
				cli.StringFlag{
					Name:     "permit",
					Usage:    "Specify contracts which are not allowed to invoke the service. If there are multiple contract addresses, separate them with ','",
					Required: false,
				},
				cli.StringFlag{
					Name:     "details",
					Usage:    "Specify service details",
					Required: true,
				},
				governanceReasonFlag,
			},
			Action: registerService,
		},
		{
			Name:  "update",
			Usage: "Update appchain service info to bitxhub",
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
					Required: false,
				},
				cli.StringFlag{
					Name:     "intro",
					Usage:    "Specify service description",
					Required: false,
				},
				cli.StringFlag{
					Name:     "permit",
					Usage:    "Specify contracts which are not allowed to invoke the service. If there are multiple contract addresses, separate them with ','",
					Required: false,
				},
				cli.StringFlag{
					Name:     "details",
					Usage:    "Specify service details",
					Required: false,
				},
				governanceReasonFlag,
				cli.BoolFlag{
					Name:  "ordered",
					Usage: "Specify if the service should be ordered",
				},
			},
			Action: updateService,
		},
		{
			Name:  "activate",
			Usage: "Activate appchain service to bitxhub",
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
				governanceReasonFlag,
			},
			Action: ActivateService,
		},
		{
			Name:  "logout",
			Usage: "Logout appchain service to bitxhub",
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
				governanceReasonFlag,
			},
			Action: LogoutService,
		},
		{
			Name:  "list",
			Usage: "List appchain service from bitxhub belong to pier self",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "appchain-id",
					Usage:    "Specify appchain ID",
					Required: true,
				},
			},
			Action: ListService,
		},
	},
}

func ListService(ctx *cli.Context) error {
	chainId := ctx.String("appchain-id")
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
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
		"GetServicesByAppchainID", nil,
		rpcx.String(chainId),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("list service info faild: %s", string(receipt.Ret))
	}
	var ret []*service_mgr.Service
	if err := json.Unmarshal(receipt.Ret, &ret); err != nil {
		return err
	}
	fmt.Printf("List appchain service by self successfully.\n %s", string(receipt.Ret))
	return nil
}

func LogoutService(ctx *cli.Context) error {
	chainID := ctx.String("appchain-id")
	serviceID := ctx.String("service-id")
	reason := ctx.String("reason")

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
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
		"LogoutService", nil,
		rpcx.String(fmt.Sprintf("%s:%s", chainID, serviceID)),
		rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("logout service info faild: %s", string(receipt.Ret))
	}
	ret := &GovernanceResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}
	fmt.Printf("Logout appchain service for %s successfully, wait for proposal %s to finish.\n", serviceID, ret.ProposalID)
	return nil
}

func ActivateService(ctx *cli.Context) error {
	chainID := ctx.String("appchain-id")
	serviceID := ctx.String("service-id")
	reason := ctx.String("reason")

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
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
		"ActivateService", nil,
		rpcx.String(fmt.Sprintf("%s:%s", chainID, serviceID)),
		rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("activate service info faild: %s", string(receipt.Ret))
	}
	ret := &GovernanceResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}
	fmt.Printf("Activate appchain service for %s successfully, wait for proposal %s to finish.\n", serviceID, ret.ProposalID)
	return nil
}

func updateService(ctx *cli.Context) error {
	chainID := ctx.String("appchain-id")
	serviceID := ctx.String("service-id")
	name := ctx.String("name")
	intro := ctx.String("intro")
	ordered := ctx.Bool("ordered")
	permit := ctx.String("permit")
	details := ctx.String("details")
	reason := ctx.String("reason")

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
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
		"UpdateService", nil,
		rpcx.String(fmt.Sprintf("%s:%s", chainID, serviceID)),
		rpcx.String(name),
		rpcx.String(intro),
		rpcx.Bool(ordered),
		rpcx.String(permit),
		rpcx.String(details),
		rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("update service info faild: %s", string(receipt.Ret))
	}
	ret := &GovernanceResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}
	if len(ret.ProposalID) == 0 {
		fmt.Printf("Update appchain service for %s successfully.\n", fmt.Sprintf("%s:%s", chainID, serviceID))
		return nil
	}
	fmt.Printf("Update appchain service for %s successfully, wait for proposal %s to finish.\n", fmt.Sprintf("%s:%s", chainID, serviceID), ret.ProposalID)
	return nil
}

func registerService(ctx *cli.Context) error {
	chainID := ctx.String("appchain-id")
	serviceID := ctx.String("service-id")
	name := ctx.String("name")
	typ := ctx.String("type")
	intro := ctx.String("intro")
	ordered := ctx.Bool("ordered")
	permit := ctx.String("permit")
	details := ctx.String("details")
	reason := ctx.String("reason")

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
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
		"RegisterService", nil,
		rpcx.String(chainID),
		rpcx.String(serviceID),
		rpcx.String(name),
		rpcx.String(typ),
		rpcx.String(intro),
		rpcx.Bool(ordered),
		rpcx.String(permit),
		rpcx.String(details),
		rpcx.String(reason),
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
	fmt.Printf("Register appchain service for %s successfully, wait for proposal %s to finish.\n", string(ret.Extra), ret.ProposalID)
	return nil
}
