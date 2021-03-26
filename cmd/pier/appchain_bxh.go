package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

type RegisterResult struct {
	ChainID    string `json:"chain_id"`
	ProposalID string `json:"proposal_id"`
}

var appchainBxhCMD = cli.Command{
	Name:  "appchain",
	Usage: "Command about appchain in bitxhub",
	Subcommands: []cli.Command{
		{
			Name:  "register",
			Usage: "Register appchain in bitxhub",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specific appchain name",
					Required: true,
				},
				cli.StringFlag{
					Name:     "type",
					Usage:    "Specific appchain type",
					Required: true,
				},
				cli.StringFlag{
					Name:     "desc",
					Usage:    "Specific appchain description",
					Required: true,
				},
				cli.StringFlag{
					Name:     "version",
					Usage:    "Specific appchain version",
					Required: true,
				},
				cli.StringFlag{
					Name:     "validators",
					Usage:    "Specific appchain validators path",
					Required: true,
				},
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specific bitxhub node address",
					Required: false,
				},
			},
			Action: registerAppchain,
		},
		{
			Name:  "get",
			Usage: "Get appchain info",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specific appchain id",
					Required: true,
				},
			},
			Action: getAppchain,
		},
	},
}

func registerAppchain(ctx *cli.Context) error {
	name := ctx.String("name")
	typ := ctx.String("type")
	desc := ctx.String("desc")
	version := ctx.String("version")
	validatorsPath := ctx.String("validators")
	bxhAddr := ctx.String("addr")

	data, err := ioutil.ReadFile(validatorsPath)
	if err != nil {
		return fmt.Errorf("read validators file: %w", err)
	}

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("init config error: %s", err)
	}

	bxhAddrs := []string{bxhAddr}
	if bxhAddr == "" {
		bxhAddrs = config.Mode.Relay.Addrs
	}
	client, err := loadClient(repo.KeyPath(repoRoot), bxhAddrs, ctx)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	pubKey, err := getPubKey(repo.KeyPath(repoRoot))
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"Register", nil, rpcx.String(string(data)),
		rpcx.Int32(1),
		rpcx.String(typ),
		rpcx.String(name),
		rpcx.String(desc),
		rpcx.String(version),
		rpcx.String(string(pubKey)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke register: %s", receipt.Ret)
	}

	ret := &RegisterResult{}
	if err := json.Unmarshal(receipt.Ret, ret); err != nil {
		return err
	}

	fmt.Printf("appchain register successfully, chain id is %s, proposal id is %s\n", ret.ChainID, ret.ProposalID)

	return nil
}

func getAppchain(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("init config error: %s", err)
	}

	client, err := loadClient(repo.KeyPath(repoRoot), config.Mode.Relay.Addrs, ctx)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"Appchain", nil,
	)

	if err != nil {
		return err
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("get appchain: %s", receipt.Ret)
	}

	fmt.Println(string(receipt.Ret))

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
	for _, addr := range grpcAddrs {
		nodeInfo := &rpcx.NodeInfo{Addr: addr}
		if config.Security.EnableTLS {
			nodeInfo.CertPath = filepath.Join(repoRoot, "certs/ca.pem")
			nodeInfo.EnableTLS = config.Security.EnableTLS
			nodeInfo.CommonName = config.Security.CommonName
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	opts = append(opts, rpcx.WithNodesInfo(nodesInfo...), rpcx.WithTimeoutLimit(config.Mode.Relay.TimeoutLimit))
	return rpcx.New(opts...)
}

func getPubKey(keyPath string) ([]byte, error) {
	privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return nil, err
	}

	return privKey.PublicKey().Bytes()
}
