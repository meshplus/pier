package main

import (
	"fmt"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
	"io/ioutil"
	"path"
)

var didCommand = cli.Command{
	Name:  "did",
	Usage: "Command about appchain did",
	Subcommands: []cli.Command{
		{
			Name:  "apply",
			Usage: "apply did method to bitxhub",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specify did method name",
					Required: true,
				},
			},
			Action: applyMethod,
		},
		{
			Name:  "register",
			Usage: "register did method to bitxhub",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "name",
					Usage:    "Specify did method name",
					Required: true,
				},
				cli.StringFlag{
					Name:     "path",
					Usage:    "Specify did method doc path",
					Required: true,
				},
			},
			Action: registerMethod,
		},
	},
}

func applyMethod(ctx *cli.Context) error {
	name := ctx.String("name")
	var sig []byte

	methodDID := "did:bitxhub:" + name + ":."

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	caller, err := getCaller(repoRoot, name)
	if err != nil {
		return fmt.Errorf("contruct caller: %w", err)
	}

	client, _, err := initClientWithKeyPath(ctx, path.Join(repoRoot, repo.KeyName))
	if err != nil {
		return err
	}

	receipt, err := client.InvokeBVMContract(
		constant.MethodRegistryContractAddr.Address(),
		"Apply", nil,
		rpcx.String(caller),
		rpcx.String(methodDID),
		rpcx.Bytes(sig),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("apply did %s to bitxhub failed: %s", methodDID, string(receipt.Ret))
	}
	fmt.Printf("apply did %s to bitxhub successfully! wait for bitxhub manager to audit.\n", methodDID)
	return nil
}

func registerMethod(ctx *cli.Context) error {
	name := ctx.String("name")
	docPath := ctx.String("path")

	methodDID := "did:bitxhub:" + name + ":."

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	caller, err := getCaller(repoRoot, name)
	if err != nil {
		return fmt.Errorf("construct caller: %w", err)
	}

	client, _, err := initClientWithKeyPath(ctx, path.Join(repoRoot, repo.KeyName))
	if err != nil {
		return err
	}

	doc, err := ioutil.ReadFile(path.Join(docPath))
	if err != nil {
		return fmt.Errorf("read method doc file: %w", err)
	}

	receipt, err := client.InvokeBVMContract(
		constant.MethodRegistryContractAddr.Address(),
		"RegisterWithDoc", nil,
		rpcx.String(caller),
		rpcx.String(methodDID),
		rpcx.Bytes(doc),
	)
	if err != nil {
		return fmt.Errorf("invoke bvm contract: %w", err)
	}
	if !receipt.IsSuccess() {
		return fmt.Errorf("register did %s to bitxhub failed: %s", methodDID, string(receipt.Ret))
	}
	fmt.Printf("register did %s to bitxhub successfully!\n", methodDID)
	return nil
}

func getCaller(repoRoot, method string) (string, error) {
	privKey, err := repo.LoadPrivateKey(repoRoot)
	if err != nil {
		return "", err
	}

	addr, err := privKey.PublicKey().Address()
	if err != nil {
		return "", err
	}

	caller := "did:bitxhub:" + method + ":" + addr.String()
	return caller, nil
}
