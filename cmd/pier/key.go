package main

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
	"io/ioutil"
	"path"
)

var keyCommand = cli.Command{
	Name:  "key",
	Usage: "Command about private key",
	Subcommands: []cli.Command{
		{
			Name:  "gen",
			Usage: "generate key file with pki file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "target",
					Usage:    "Specify target directory, default repoRoot",
					Required: false,
				},
				cli.StringFlag{
					Name:     "path",
					Usage:    "Specify pki private key file path",
					Required: true,
				},
				cli.StringFlag{
					Name:     "passwd",
					Usage:    "Specify password, default \"bitxhub\"",
					Required: false,
				},
			},
			Action: genPrivateKey,
		},
		{
			Name:  "address",
			Usage: "show address with pki file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "path",
					Usage:    "Specify pki public key file path",
					Required: true,
				},
			},
			Action: showAddress,
		},
	},
}

func genPrivateKey(ctx *cli.Context) error {
	type ecPrivateKey struct {
		Version       int
		PrivateKey    []byte
		NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
		PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
	}
	target := ctx.String("target")
	pkiPath := ctx.String("path")
	passwd := ctx.String("passwd")

	if len(target) == 0 {
		repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
		if err != nil {
			return err
		}
		target = path.Join(repoRoot, repo.KeyName)
	}
	if len(passwd) == 0 {
		passwd = repo.KeyPassword
	}

	privData, err := ioutil.ReadFile(pkiPath)
	if err != nil {
		return fmt.Errorf("read pki file: %w", err)
	}

	b, _ := pem.Decode(privData)
	if b == nil {
		return fmt.Errorf("empty block")
	}

	var privKey ecPrivateKey
	if _, err := asn1.Unmarshal(b.Bytes, &privKey); err != nil {
		return err
	}

	priv, err := ecdsa.UnmarshalPrivateKey(privKey.PrivateKey, crypto.Secp256k1)
	if err != nil {
		return fmt.Errorf("unmarshal private key: %w", err)
	}

	if err := asym.StorePrivateKey(priv, target, passwd); err != nil {
		return fmt.Errorf("store key file: %w", err)
	}
	return nil
}

func showAddress(ctx *cli.Context) error {
	type publicKeyInfo struct {
		Raw       asn1.RawContent
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	pkiPath := ctx.String("path")
	pubKeyData, err := ioutil.ReadFile(pkiPath)
	if err != nil {
		return fmt.Errorf("read pki file: %w", err)
	}

	b, _ := pem.Decode(pubKeyData)
	if b == nil {
		return fmt.Errorf("empty block")
	}

	var pki publicKeyInfo
	if _, err := asn1.Unmarshal(b.Bytes, &pki); err != nil {
		return err
	}

	pubKey, err := ecdsa.UnmarshalPublicKey(pki.PublicKey.RightAlign(), crypto.Secp256k1)
	if err != nil {
		return fmt.Errorf("unmarshal public key: %w", err)
	}

	addr, err := pubKey.Address()
	if err != nil {
		return err
	}
	fmt.Println(addr.String())
	return nil
}
