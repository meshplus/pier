package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/hexutil"
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
			Name:  "update",
			Usage: "update appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specify appchain id(did)",
					Required: true,
				},
				cli.StringFlag{
					Name:     "doc-addr",
					Usage:    "Specify appchain did doc addr",
					Required: false,
				},
				cli.StringFlag{
					Name:     "doc-hash",
					Usage:    "Specify appchain did doc hash",
					Required: false,
				},
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specify appchain name",
					Required: false,
				},
				cli.StringFlag{
					Name:     "type",
					Usage:    "Specify appchain type",
					Required: false,
				},
				cli.StringFlag{
					Name:     "desc",
					Usage:    "Specify appchain description",
					Required: false,
				},
				cli.StringFlag{
					Name:     "version",
					Usage:    "Specify appchain version",
					Required: false,
				},
				cli.StringFlag{
					Name:     "validators",
					Usage:    "Specify appchain validators path",
					Required: false,
				},
				cli.StringFlag{
					Name:     "consensus-type",
					Usage:    "Specify appchain consensus type",
					Required: false,
				},
			},
			Action: updateAppchain,
		},
		{
			Name:  "freeze",
			Usage: "freeze appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specify appchain id(did)",
					Required: true,
				},
			},
			Action: freezeAppchain,
		},
		{
			Name:  "activate",
			Usage: "activate appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specify appchain id(did)",
					Required: true,
				},
			},
			Action: activateAppchain,
		},
		{
			Name:  "logout",
			Usage: "logout appchain in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specify appchain id(did)",
					Required: true,
				},
			},
			Action: logoutAppchain,
		},
		{
			Name:  "get",
			Usage: "Get appchain info",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specify appchain id(did)",
					Required: true,
				},
			},
			Action: getAppchain,
		},
	},
}

func registerPier(ctx *cli.Context) error {
	// todo: add register pier logic
	return nil
}

func updateAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("id")
	docAddr := ctx.String("doc-addr")
	docHash := ctx.String("doc-hash")
	name := ctx.String("name")
	typ := ctx.String("type")
	desc := ctx.String("desc")
	version := ctx.String("version")
	validatorsPath := ctx.String("validators")
	consensusType := ctx.String("consensus-type")

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
	if docAddr == "" {
		docAddr = appchainInfo.DidDocAddr
	}
	if docHash == "" {
		docHash = appchainInfo.DidDocHash
	}
	if name == "" {
		name = appchainInfo.Name
	}
	if typ == "" {
		typ = appchainInfo.ChainType
	}
	if desc == "" {
		desc = appchainInfo.Desc
	}
	if version == "" {
		version = appchainInfo.Version
	}
	validators := ""
	if validatorsPath == "" {
		validators = appchainInfo.Validators
	} else {
		data, err := ioutil.ReadFile(validatorsPath)
		if err != nil {
			return fmt.Errorf("read validators file: %w", err)
		}
		validators = string(data)
	}
	if consensusType == "" {
		consensusType = appchainInfo.ConsensusType
	}

	pubKey, err := getPubKey(chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	receipt, err = client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"UpdateAppchain", nil,
		rpcx.String(id),
		rpcx.String(docAddr),
		rpcx.String(docHash),
		rpcx.String(validators),
		rpcx.String(consensusType),
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

func freezeAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("id")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"FreezeAppchain", nil, rpcx.String(id),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke freeze: %s", receipt.Ret)
	}

	proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
	fmt.Printf("the freeze request was submitted successfully, proposal id is %s\n", proposalId)

	return nil
}

func activateAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("id")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"ActivateAppchain", nil, rpcx.String(id),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}

	if !receipt.IsSuccess() {
		return fmt.Errorf("invoke activate: %s", receipt.Ret)
	}

	proposalId := gjson.Get(string(receipt.Ret), "proposal_id").String()
	fmt.Printf("the activate request was submitted successfully, proposal id is %s\n", proposalId)

	return nil
}

func logoutAppchain(ctx *cli.Context) error {
	chainAdminKeyPath := ctx.String("admin-key")
	id := ctx.String("id")

	client, _, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"LogoutAppchain", nil, rpcx.String(id),
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
	id := ctx.String("id")

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
	appchainInfo.PublicKey = hexutil.Encode([]byte(appchainInfo.PublicKey))

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
