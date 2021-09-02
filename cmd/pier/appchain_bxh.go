package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli"
)

var appchainBxhCMD = cli.Command{
	Name:  "appchain",
	Usage: "Command about appchain in bitxhub",
	Subcommands: []cli.Command{
		//methodCommand,
		serviceCommand,
		didCommand,
		{
			Name:  "register",
			Usage: "Register appchain to bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				appchainIdFlag,
				appchainTrustRootFlag,
				appchainBrokerFlag,
				appchainDescFlag,
				appchainMasterRuleFlag,
				governanceReasonFlag,
			},
			Action: registerPier,
		},
		{
			Name:  "update",
			Usage: "update appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				appchainIdFlag,
				cli.StringFlag{
					Name:     "desc",
					Usage:    "Specify appchain description",
					Required: false,
				},
			},
			Action: updateAppchain,
		},
		{
			Name:  "activate",
			Usage: "activate appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				appchainIdFlag,
				governanceReasonFlag,
			},
			Action: activateAppchain,
		},
		{
			Name:  "logout",
			Usage: "logout appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				appchainIdFlag,
				governanceReasonFlag,
			},
			Action: logoutAppchain,
		},
		{
			Name:  "get",
			Usage: "Get appchain info",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				appchainIdFlag,
			},
			Action: getAppchain,
		},
	},
}

func registerPier(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("appchain-id")
	trustrootPath := ctx.String("trustroot")
	broker := ctx.String("broker")
	desc := ctx.String("desc")
	masterRule := ctx.String("master-rule")
	reason := ctx.String("reason")
	trustrootData, err := ioutil.ReadFile(trustrootPath)
	if err != nil {
		return fmt.Errorf("read validators file: %w", err)
	}

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"RegisterAppchain", nil,
		rpcx.String(id),
		rpcx.Bytes(trustrootData),
		rpcx.String(broker),
		rpcx.String(desc),
		rpcx.String(masterRule),
		rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("register appchain faild: %s", string(receipt.Ret))
	}
	ret := &GovernanceResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}
	fmt.Printf("Register appchain  for %s successfully, wait for proposal %s to finish.\n", string(ret.Extra), ret.ProposalID)
	return nil
}

func updateAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("appchain-id")
	desc := ctx.String("desc")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"GetAppchain", nil, rpcx.String(id),
	)
	if err != nil {
		return err
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("get appchain: %s", receipt.Ret)
	}

	appchainInfo := appchainmgr.Appchain{}
	if err = json.Unmarshal(receipt.Ret, &appchainInfo); err != nil {
		return err
	}
	if desc == "" {
		desc = appchainInfo.Desc
	}
	receipt, err = client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"UpdateAppchain", nil,
		rpcx.String(id),
		rpcx.String(desc),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke update: %s", receipt.Ret)
	}

	proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
	if proposalId != "" {
		fmt.Printf("the update request was submitted successfully, proposal id is %s\n", proposalId)
	} else {
		fmt.Printf("the update request was submitted successfully\n")
	}

	return nil
}

func activateAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("appchain-id")
	reason := ctx.String("reason")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"ActivateAppchain", nil, rpcx.String(id), rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke activate: %s", receipt.Ret)
	}

	proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
	if proposalId != "" {
		fmt.Printf("the activate request was submitted successfully, proposal id is %s\n", proposalId)
	} else {
		fmt.Printf("the activate request was submitted successfully\n")
	}

	return nil
}

func logoutAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("appchain-id")
	reason := ctx.String("reason")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"LogoutAppchain", nil, rpcx.String(id), rpcx.String(reason),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke logout: %s", receipt.Ret)
	}

	proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
	fmt.Printf("the logout request was submitted successfully, proposal id is %s\n", proposalId)

	return nil
}

func getAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("appchain-id")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"GetAppchain", nil, rpcx.String(id),
	)

	if err != nil {
		return err
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("get appchain: %s", receipt.Ret)
	}

	appchainInfo := appchainmgr.Appchain{}
	if err = json.Unmarshal(receipt.Ret, &appchainInfo); err != nil {
		return err
	}

	appchainData, err := json.Marshal(appchainInfo)
	if err != nil {
		return err
	}
	fmt.Println(string(appchainData))

	return nil
}

func loadClient(keyPath string, grpcAddrs []string, ctx *cli.Context) (rpcx.Client, error) {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return nil, err
	}

	repo.SetPath(repoRoot)

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("init config error: %s", err)
	}

	privateKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return nil, err
	}

	opts := []rpcx.Option{
		rpcx.WithPrivateKey(privateKey),
	}
	nodesInfo := make([]*rpcx.NodeInfo, 0, len(grpcAddrs))
	for index, addr := range grpcAddrs {
		nodeInfo := &rpcx.NodeInfo{Addr: addr}
		if config.Security.EnableTLS {
			nodeInfo.CertPath = filepath.Join(repoRoot, config.Security.Tlsca)
			nodeInfo.EnableTLS = config.Security.EnableTLS
			nodeInfo.CommonName = config.Security.CommonName
			nodeInfo.AccessCert = filepath.Join(config.RepoRoot, config.Security.AccessCert[index])
			nodeInfo.AccessKey = filepath.Join(config.RepoRoot, config.Security.AccessKey)
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	opts = append(opts, rpcx.WithNodesInfo(nodesInfo...), rpcx.WithTimeoutLimit(config.Mode.Relay.TimeoutLimit))
	return rpcx.New(opts...)
}

func getPubKey(keyPath string) (string, error) {
	privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return "", err
	}

	pubBytes, err := privKey.PublicKey().Bytes()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pubBytes), nil
}

func getAddr(keyPath string) (string, error) {
	privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return "", err
	}

	addr, err := privKey.PublicKey().Address()
	if err != nil {
		return "", err
	}

	return addr.String(), nil
}
