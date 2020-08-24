package main

import (
	"crypto"
	"fmt"

	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var p2pCMD = cli.Command{
	Name:  "p2p",
	Usage: "Command about p2p",
	Subcommands: []cli.Command{
		{
			Name:   "id",
			Usage:  "get pier unique id in p2p network",
			Action: p2pID,
		},
	},
}

func p2pID(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	privKey, err := repo.LoadNodePrivateKey(repoRoot)
	if err != nil {
		return err
	}

	stdKey, err := asym.PrivKeyToStdKey(privKey)
	if err != nil {
		return err
	}

	_, pk, err := crypto2.KeyPairFromStdKey(&stdKey)
	if err != nil {
		return err
	}

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return err
	}

	fmt.Println(id)
	return nil
}

func convertToLibp2pPrivKey(privateKey crypto.PrivateKey) (crypto2.PrivKey, error) {
	ecdsaPrivKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("convert to libp2p private key: not ecdsa private key")
	}

	libp2pPrivKey, _, err := crypto2.ECDSAKeyPairFromKey(ecdsaPrivKey.K)
	if err != nil {
		return nil, err
	}

	return libp2pPrivKey, nil
}
