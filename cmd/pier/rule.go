package main

import (
	"fmt"
	"io/ioutil"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
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
				methodFlag,
				adminKeyPathFlag,
			},
			Action: deployRule,
		},
		{
			Name:  "bind",
			Usage: "bind validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific rule addr",
					Required: true,
				},
				methodFlag,
				adminKeyPathFlag,
			},
			Action: bindRule,
		},
		{
			Name:  "unbind",
			Usage: "unbind validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific rule addr",
					Required: true,
				},
				methodFlag,
				adminKeyPathFlag,
			},
			Action: unbindRule,
		},
		{
			Name:  "freeze",
			Usage: "freeze validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific rule addr",
					Required: true,
				},
				methodFlag,
				adminKeyPathFlag,
			},
			Action: freezeRule,
		},
		{
			Name:  "activate",
			Usage: "activate validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific rule addr",
					Required: true,
				},
				methodFlag,
				adminKeyPathFlag,
			},
			Action: activateRule,
		},
		{
			Name:  "logout",
			Usage: "logout validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific rule addr",
					Required: true,
				},
				methodFlag,
				adminKeyPathFlag,
			},
			Action: logoutRule,
		},
	},
}

func deployRule(ctx *cli.Context) error {
	rulePath := ctx.String("path")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	contract, err := ioutil.ReadFile(rulePath)
	if err != nil {
		return err
	}

	contractAddr, err := client.DeployContract(contract, nil)
	if err != nil {
		color.Red("deploy rule error: %w", err)
	} else {
		color.Green(fmt.Sprintf("Deploy rule to bitxhub for appchain %s successfully: %s", method, contractAddr.String()))
	}

	return nil
}

func bindRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"BindRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("Bind rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Bind rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Bind rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, string(receipt.Ret)))
	}

	return nil
}

func unbindRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"UnbindRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("Unind rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Unbind rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Unbind rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, string(receipt.Ret)))
	}

	return nil
}

func freezeRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"FreezeRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("freeze rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Freeze rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Freeze rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, string(receipt.Ret)))
	}

	return nil
}

func activateRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"ActivateRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("activate rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Activate rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Activate rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, string(receipt.Ret)))
	}

	return nil
}

func logoutRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"LogoutRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("logout rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Logout rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Logout rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, string(receipt.Ret)))
	}

	return nil
}
