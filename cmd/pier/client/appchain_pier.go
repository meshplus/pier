package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/meshplus/pier/internal/rulemgr"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/key"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-kit/wasm"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

type Approve struct {
	Id         string `json:"id"`
	IsApproved int32  `json:"is_approved"`
	Desc       string `json:"desc"`
}

var clientCMD = cli.Command{
	Name:  "client",
	Usage: "Command about appchain in pier",
	Subcommands: []cli.Command{
		{
			Name:  "register",
			Usage: "Register appchain in pier",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "pier_id",
					Usage:    "Specific target pier id",
					Required: true,
				},
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
			},
			Action: registerPierAppchain,
		},
		{
			Name:  "update",
			Usage: "Update appchain in pier",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "pier",
					Usage:    "Specific target pier id",
					Required: true,
				},
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
			},
			Action: updatePierAppchain,
		},
		{
			Name:  "audit",
			Usage: "Audit appchain in pier",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specific appchain id",
					Required: true,
				},
				cli.StringFlag{
					Name:     "isApproved",
					Usage:    "Specific approved signal",
					Required: true,
				},
				cli.StringFlag{
					Name:     "desc",
					Usage:    "Specific audit description",
					Required: true,
				},
			},
			Action: auditPierAppchain,
		},
		{
			Name:  "get",
			Usage: "Get appchain info",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "pier",
					Usage:    "Specific target pier id",
					Required: true,
				},
				cli.StringFlag{
					Name:     "id",
					Usage:    "Specific appchain id",
					Required: true,
				},
			},
			Action: getPierAppchain,
		},
		{
			Name:  "rule",
			Usage: "register appchain validation rule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "path",
					Usage:    "rule file path",
					Required: true,
				},
				cli.StringFlag{
					Name:     "pier",
					Usage:    "Specific target pier id",
					Required: true,
				},
			},
			Action: registerAppchainRule,
		},
	},
}

func LoadClientCMD() cli.Command {
	return clientCMD
}

func registerPierAppchain(ctx *cli.Context) error {
	return savePierAppchain(ctx, RegisterAppchainUrl)
}

func updatePierAppchain(ctx *cli.Context) error {
	return savePierAppchain(ctx, UpdateAppchainUrl)
}

func auditPierAppchain(ctx *cli.Context) error {
	id := ctx.String("id")
	isApproved := ctx.String("isApproved")
	desc := ctx.String("desc")

	ia, err := strconv.ParseInt(isApproved, 0, 64)
	if err != nil {
		return fmt.Errorf("isApproved must be 0 or 1: %w", err)
	}
	approve := &Approve{
		Id:         id,
		IsApproved: int32(ia),
		Desc:       desc,
	}

	data, err := json.Marshal(approve)
	if err != nil {
		return err
	}
	url, err := getURL(ctx, AuditAppchainUrl)
	if err != nil {
		return err
	}

	_, err = httpPost(url, data)
	if err != nil {
		return err
	}

	fmt.Printf("audit appchain %s successfully\n", id)

	return nil
}

func savePierAppchain(ctx *cli.Context, path string) error {
	pier := ctx.String("pier")
	name := ctx.String("name")
	typ := ctx.String("type")
	desc := ctx.String("desc")
	version := ctx.String("version")
	validatorsPath := ctx.String("validators")

	data, err := ioutil.ReadFile(validatorsPath)
	if err != nil {
		return fmt.Errorf("read validators file: %w", err)
	}

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	pubKey, err := getPubKey(repo.KeyPath(repoRoot))
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}
	addr, _ := pubKey.Address()
	pubKeyBytes, _ := pubKey.Bytes()
	appchain := appchainmgr.Appchain{
		ID:            addr.String(),
		Name:          name,
		Validators:    string(data),
		ConsensusType: 1,
		ChainType:     typ,
		Desc:          desc,
		Version:       version,
		PublicKey:     string(pubKeyBytes),
	}

	data, err = json.Marshal(appchain)
	if err != nil {
		return fmt.Errorf("marshal appchain error: %w", err)
	}

	url, err := getURL(ctx, fmt.Sprintf("%s?pier=%s", path, pier))
	if err != nil {
		return err
	}
	resp, err := httpPost(url, data)
	if err != nil {
		return err
	}

	fmt.Println(parseResponse(resp))

	return nil
}

func getPierAppchain(ctx *cli.Context) error {
	pier := ctx.String("pier")
	id := ctx.String("id")

	url, err := getURL(ctx, fmt.Sprintf("%s?pier=%s&id=%s", GetAppchainUrl, pier, id))
	if err != nil {
		return err
	}
	res, err := httpGet(url)
	if err != nil {
		return err
	}
	fmt.Println(parseResponse(res))

	return nil
}

func getPubKey(keyPath string) (crypto.PublicKey, error) {
	key, err := key.LoadKey(keyPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := key.GetPrivateKey("bitxhub")
	if err != nil {
		return nil, err
	}

	return privateKey.PublicKey(), nil
}

func registerAppchainRule(ctx *cli.Context) error {
	path := ctx.String("path")
	pier := ctx.String("pier")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read rule file: %w", err)
	}
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	pubKey, err := getPubKey(repo.KeyPath(repoRoot))
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}
	addr, _ := pubKey.Address()

	contract := wasm.Contract{
		Code: data,
		Hash: types.Bytes2Hash(data),
	}

	code, err := json.Marshal(contract)
	if err != nil {
		return fmt.Errorf("marshal contarct: %w", err)
	}

	rule := rulemgr.Rule{
		Code:    code,
		Address: addr.String(),
	}
	postData, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal rule error: %w", err)
	}

	url, err := getURL(ctx, fmt.Sprintf("%s?pier=%s", RegisterRuleUrl, pier))
	if err != nil {
		return err
	}

	resp, err := httpPost(url, postData)
	if err != nil {
		return err
	}

	fmt.Println(parseResponse(resp))

	return nil
}
