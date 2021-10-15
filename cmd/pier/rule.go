package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/meshplus/bitxhub-model/constant"
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
					Usage:    "Specify rule path",
					Required: true,
				},
			},
			Action: deployRule,
		},
		{
			Name:  "register",
			Usage: "Register validation rule",
			Flags: []cli.Flag{
				appchainIdFlag,
				appchainRuleFlag,
				appchainRuleUrlFlag,
			},
			Action: registerRule,
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
			},
			Action: logoutRule,
		},
	},
}

func deployRule(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	rulePath := ctx.String("path")

	keyPath := filepath.Join(repoRoot, "key.json")
	client, _, err := initClientWithKeyPath(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	contract, err := ioutil.ReadFile(rulePath)
	if err != nil {
		return err
	}

	contractAddr, err := client.DeployContract(contract, nil)
	if err != nil {
		return fmt.Errorf("Deploy rule error: %w", err)
	} else {
		color.Green(fmt.Sprintf("Deploy rule successfully: %s", contractAddr.String()))
		return nil
	}
}

func registerRule(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	appchainID := ctx.String("appchain-id")
	rule_addr := ctx.String("rule")
	rule_url := ctx.String("rule-url")

	keyPath := filepath.Join(repoRoot, "key.json")
	client, _, err := initClientWithKeyPath(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("Load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.RuleManagerContractAddr.Address(),
		"RegisterRule", nil,
		rpcx.String(appchainID), rpcx.String(rule_addr), rpcx.String(rule_url))
	if err != nil {
		return fmt.Errorf("Register rule: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("Register rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret))
	} else {
		color.Green(fmt.Sprintf("Register rule to bitxhub for appchain %s successfully.", appchainID))
		return nil
	}
}

func updateMasterRule(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	ruleAddr := ctx.String("addr")
	appchainID := ctx.String("appchain-id")
	reason := ctx.String("reason")

	keyPath := filepath.Join(repoRoot, "key.json")
	client, _, err := initClientWithKeyPath(ctx, keyPath)
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
		return fmt.Errorf("Update master rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret))
	} else {
		proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
		color.Green(fmt.Sprintf("Update master rule to bitxhub for appchain %s successfully, wait for proposal %s to finish.", appchainID, proposalId))
		return nil
	}
}
func logoutRule(ctx *cli.Context) error {
	ruleAddr := ctx.String("addr")
	appchainID := ctx.String("appchain-id")
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	keyPath := filepath.Join(repoRoot, "key.json")
	client, _, err := initClientWithKeyPath(ctx, keyPath)
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
		return fmt.Errorf("Logout rule to bitxhub for appchain %s error: %s", appchainID, string(receipt.Ret))
	} else {
		color.Green("The logout request was submitted successfully\n")
		return nil
	}
}
