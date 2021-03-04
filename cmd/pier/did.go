package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/bitxhub/bitxid"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

const (
	bitxhubRootPrefix  = "did:bitxhub"
	relayRootSubMethod = "relayroot"
	fakeSignature      = "fake signature"
	fakeDocAddr        = "/ipfs/QmQVxzUqN2Yv2UHUQXYwH8dSNkM8ReJ9qPqwJsf8zzoNUi"
	fakeDocHash        = "QmQVxzUqN2Yv2UHUQXYwH8dSNkM8ReJ9qPqwJsf8zzoNUi"
)

var methodCommand = cli.Command{
	Name:  "method",
	Usage: "Command about appchain method",
	Subcommands: []cli.Command{
		{
			Name:  "apply",
			Usage: "Apply appchain method in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				methodFlag,
			},
			Action: applyMethod,
		},
		{
			Name:  "audit",
			Usage: "Audit registered appchain method in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				methodFlag,
				statusFlag,
			},
			Action: auditMethod,
		},
		{
			Name:  "register",
			Usage: "Register appchain doc info to appchain did in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				methodFlag,
				didDocAddrFlag,
				didDocHashFlag,
				appchainNameFlag,
				appchainTypeFlag,
				appchainDescFlag,
				appchainVersionFlag,
				appchainValidatorFlag,
			},
			Action: registerMethod,
		},
		{
			Name:  "auditInfo",
			Usage: "Audit registered appchain method info in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				methodFlag,
				statusFlag,
			},
			Action: auditMethodInfo,
		},
	},
}

var didCommand = cli.Command{
	Name:  "did",
	Usage: "Command about appchain did",
	Subcommands: []cli.Command{
		{
			Name:  "register",
			Usage: "Register appchain did in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				didFlag,
			},
			Action: registerDID,
		},
		{
			Name:  "audit",
			Usage: "Audit registered appchain did info in bitxhub",
			Flags: []cli.Flag{
				adminKeyPathFlag,
				didFlag,
				statusFlag,
			},
			Action: auditDID,
		},
	},
}

func applyMethod(ctx *cli.Context) error {
	appchainSubMethod := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")

	client, address, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}
	appchainAdminDID := fmt.Sprintf("%s:%s:%s", bitxhubRootPrefix, appchainSubMethod, address.String())
	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, appchainSubMethod)
	_, err = client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"Apply", nil, rpcx.String(appchainAdminDID),
		rpcx.String(appchainMethod), rpcx.Bytes([]byte(fakeSignature)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	fmt.Printf("Apply method %s with admin did %s successfully\n", appchainMethod, appchainAdminDID)
	return nil
}

func auditMethod(ctx *cli.Context) error {
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")
	status := ctx.Int("status")

	client, address, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}
	relayAdminDID := fmt.Sprintf("%s:%s:%s", bitxhubRootPrefix, relayRootSubMethod, address.String())
	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	_, err = client.InvokeBVMContract(
		constant.MethodRegistryContractAddr.Address(),
		"AuditApply", nil, rpcx.String(relayAdminDID),
		rpcx.String(appchainMethod), rpcx.Int32(int32(status)),
		rpcx.Bytes([]byte(fakeSignature)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	fmt.Printf("Audit method %s with admin did %s to status %d successfully\n", appchainMethod, relayAdminDID, status)
	return nil
}

func registerMethod(ctx *cli.Context) error {
	method := ctx.String("method")
	chainAdminKeyPath := ctx.String("admin-key")
	didDocAddr := ctx.String("doc-addr")
	name := ctx.String("name")
	typ := ctx.String("type")
	desc := ctx.String("desc")
	version := ctx.String("version")
	validatorsPath := ctx.String("validators")
	validatorData, err := ioutil.ReadFile(validatorsPath)
	if err != nil {
		return fmt.Errorf("read validators file: %w", err)
	}
	// get doc hash from doc addr postfix
	multiAddr := strings.Split(didDocAddr, "/")
	if len(multiAddr) != 3 || multiAddr[1] != "ipfs" {
		return fmt.Errorf("did doc address %s is not in ipfs address format", didDocAddr)
	}
	didDocHash := multiAddr[len(multiAddr)-1]

	// get repo public key
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	pubKey, err := getPubKey(repo.KeyPath(repoRoot))
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	client, address, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}
	appchainAdminDID := fmt.Sprintf("%s:%s:%s", bitxhubRootPrefix, method, address.String())
	appchainMethod := fmt.Sprintf("%s:%s:.", bitxhubRootPrefix, method)
	// init method registry with this admin key
	_, err = client.InvokeBVMContract(
		constant.AppchainMgrContractAddr.Address(),
		"Register", nil, rpcx.String(appchainAdminDID),
		rpcx.String(appchainMethod), rpcx.Bytes([]byte(fakeSignature)),
		rpcx.String(didDocAddr), rpcx.String(didDocHash),
		rpcx.String(string(validatorData)), rpcx.String(typ),
		rpcx.String(name), rpcx.String(desc), rpcx.String(version),
		rpcx.String(string(pubKey)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	fmt.Printf("Register method doc info for %s successfully\n", appchainMethod)
	return nil
}

func auditMethodInfo(ctx *cli.Context) error {
	// todo: wait for audit method info api in bitxhub to implement
	return nil
}

func registerDID(ctx *cli.Context) error {
	did := ctx.String("did")
	chainAdminKeyPath := ctx.String("admin-key")

	client, address, err := initClientWithKeyPath(ctx, chainAdminKeyPath)
	if err != nil {
		return err
	}
	appchainDID := bitxid.DID(did)
	method := appchainDID.GetRootMethod()
	appchainAdminDID := fmt.Sprintf("%s:%s:%s", bitxhubRootPrefix, method, address.String())
	_, err = client.InvokeBVMContract(
		constant.DIDRegistryContractAddr.Address(),
		"Register", nil, rpcx.String(appchainAdminDID),
		rpcx.String(did), rpcx.String(fakeDocAddr),
		rpcx.String(fakeDocHash), rpcx.Bytes([]byte(fakeSignature)),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	fmt.Printf("Register did doc info for %s successfully\n", did)
	return nil
}

func auditDID(ctx *cli.Context) error {
	// todo: wait for audit did info api in bitxhub to implement
	return nil
}

func initClientWithKeyPath(ctx *cli.Context, chainAdminKeyPath string) (rpcx.Client, *types.Address, error) {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return nil, nil, err
	}

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("init config error: %s", err)
	}

	adminPriv, err := asym.RestorePrivateKey(chainAdminKeyPath, "bitxhub")
	if err != nil {
		return nil, nil, err
	}
	address, err := adminPriv.PublicKey().Address()
	if err != nil {
		return nil, nil, err
	}

	client, err := loadClient(chainAdminKeyPath, config.Mode.Relay.Addrs, ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load client: %w", err)
	}
	return client, address, nil
}
