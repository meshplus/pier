package txcrypto

import (
	"fmt"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/crypto/ecdh"
	"github.com/meshplus/bitxhub-kit/crypto/sym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type RelayCryptor struct {
	client  rpcx.Client
	privKey crypto.PrivateKey
	keyMap  map[string][]byte
}

func NewRelayCryptor(client rpcx.Client, privKey crypto.PrivateKey) (Cryptor, error) {
	keyMap := make(map[string][]byte)

	return &RelayCryptor{
		client:  client,
		privKey: privKey,
		keyMap:  keyMap,
	}, nil
}

func (c *RelayCryptor) Encrypt(content []byte, address string) ([]byte, error) {
	des, err := c.getDesKey(address)
	if err != nil {
		return nil, err
	}
	return des.Encrypt(content)
}

func (c *RelayCryptor) Decrypt(content []byte, address string) ([]byte, error) {
	des, err := c.getDesKey(address)
	if err != nil {
		return nil, err
	}
	return des.Decrypt(content)
}

func (c *RelayCryptor) getDesKey(address string) (crypto.SymmetricKey, error) {
	pubKey, ok := c.keyMap[address]
	if !ok {
		ret, err := c.client.InvokeBVMContract(constant.AppchainMgrContractAddr.Address(), "GetPubKeyByChainID", nil, rpcx.String(address))
		if err != nil {
			return nil, err
		}
		c.keyMap[address] = ret.Ret
		pubKey = ret.Ret
	}
	ke, err := ecdh.NewEllipticECDH(ecdsa.S256())
	if err != nil {
		return nil, err
	}
	fmt.Println("99")
	secret, err := ke.ComputeSecret(c.privKey, pubKey)
	if err != nil {
		return nil, err
	}
	fmt.Println("888")
	return sym.GenerateSymKey(crypto.ThirdDES, secret)
}
