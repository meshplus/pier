package main

import (
	"fmt"
	"path/filepath"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var appchainBxhCMD = cli.Command{
	Name:  "appchain",
	Usage: "Command about appchain in bitxhub",
	Subcommands: []cli.Command{
		methodCommand,
		didCommand,
		{
			Name:  "register",
			Usage: "Register pier to bitxhub",
			Flags: []cli.Flag{
				methodFlag,
			},
			Action: registerPier,
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
		{
			Name:  "init",
			Usage: "Init did registry admin in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
			},
			Action: initAdminDID,
		},
	},
}

func registerPier(ctx *cli.Context) error {
	// todo: add register pier logic
	return nil
}

func initAdminDID(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")

	client, address, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}
	relayAdminDID := fmt.Sprintf("%s:%s:%s", bitxhubRootPrefix, relayRootSubMethod, address.String())
	// init method registry with this admin key
	_, err = client.InvokeBVMContract(
		constant.MethodRegistryContractAddr.Address(),
		"Init", nil, rpcx.String(relayAdminDID),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	// init did registry with this admin key
	_, err = client.InvokeBVMContract(
		constant.DIDRegistryContractAddr.Address(),
		"Init", nil, rpcx.String(relayAdminDID),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	fmt.Printf("Init method and did registry with admin did %s successfully\n", relayAdminDID)
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
	opts = append(opts, rpcx.WithNodesInfo(nodesInfo...))
	return rpcx.New(opts...)
}

func getPubKey(keyPath string) ([]byte, error) {
	privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return nil, err
	}

	return privKey.PublicKey().Bytes()
}
