package main

import (
	"fmt"
	"io/ioutil"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
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
					Usage:    "Specify rule path",
					Required: true,
				},

				appchainIdFlag,
				adminKeyPathFlag,
				appchainRuleUrlFlag,
			},
			Action: deployRule,
		},
		{
			Name:  "update",
			Usage: "update master rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specify rule addr",
					Required: true,
				},
				appchainIdFlag,
				adminKeyPathFlag,
				governanceReasonFlag,
			},
			Action: updateMasterRule,
		},
		{
			Name:  "logout",
			Usage: "logout validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specify rule addr",
					Required: true,
				},
				appchainIdFlag,
				adminKeyPathFlag,
			},
			Action: logoutRule,
		},
	},
}

func deployRule(ctx *cli.Context) error {
	rulePath := ctx.String("path")
	appchainID := ctx.String("appchain-id")
	chainAdminKeyPath := ctx.String("admin-key")
	rule_url := ctx.String("rule-url")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	contract, err := ioutil.ReadFile(rulePath)
	if err != nil {
		return err
	}

	// 1. deploy
	contractAddr, err := client.DeployContract(contract, nil)
	if err != nil {
		color.Red("Deploy rule error: %w", err)
		return nil
	} else {
		color.Green(fmt.Sprintf("Deploy rule to bitxhub for appchain %s successfully: %s", appchainID, contractAddr.String()))
	}

	// 2. register
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"RegisterRule", nil,
		rpcx.String(appchainID), rpcx.String(contractAddr.String()), rpcx.String(rule_url))
	if err != nil {
		return fmt.Errorf("Register rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Register rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret)))
	} else {
		color.Green(fmt.Sprintf("Register rule to bitxhub for appchain %s successfully.", appchainID))
	}

	return nil
}

func updateMasterRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	appchainID := ctx.String("appchain-id")
	chainAdminKeyPath := ctx.String("admin-key")
	reason := ctx.String("reason")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"UpdateMasterRule", nil,
		rpcx.String(appchainID), rpcx.String(ruleAddr), rpcx.String(reason))
	if err != nil {
		return fmt.Errorf("Update master rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Update master rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret)))
	} else {
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Update master rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainID, proposalId))
	}

	return nil
}

func bindRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
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
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Bind rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, proposalId))
	}

	return nil
}

func unbindRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
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
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Unbind rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, proposalId))
	}

	return nil
}

func freezeRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"FreezeRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("Freeze rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Freeze rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Freeze rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, proposalId))
	}

	return nil
}

func activateRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"ActivateRule", nil,
		rpcx.String(appchainMethod), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("Activate rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Activate rule to bitxhub for appchain %s error: %s", appchainMethod, string(receipt.Ret)))
	} else {
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Activate rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainMethod, proposalId))
	}

	return nil
}

func logoutRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	appchainID := ctx.String("appchain-id")
	chainAdminKeyPath := ctx.String("admin-key")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"LogoutRule", nil,
		rpcx.String(appchainID), rpcx.String(ruleAddr))
	if err != nil {
		return fmt.Errorf("Logout rule: %w", err)
	}

	if !receipt.IsSuccess() {
		color.Red(fmt.Sprintf("Logout rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret)))
	} else {
		color.Green("The logout request was submitted successfully\n")
	}

	return nil
}
