package txcrypto

import (
	"encoding/base64"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/crypto/ecdh"
	"github.com/meshplus/bitxhub-kit/crypto/sym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type RelayCryptor struct {
	client      rpcx.Client
	privKey     crypto.PrivateKey
	keyMap      map[string][]byte
	isEncrypted bool
}

func NewRelayCryptor(client rpcx.Client, privKey crypto.PrivateKey, isEncrypted bool) (Cryptor, error) {
	keyMap := make(map[string][]byte)

	return &RelayCryptor{
		client:      client,
		privKey:     privKey,
		keyMap:      keyMap,
		isEncrypted: isEncrypted,
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

func (c *RelayCryptor) IsPrivacy() bool {
	return c.isEncrypted
}

func (c *RelayCryptor) getDesKey(address string) (crypto.SymmetricKey, error) {
	pubKey, ok := c.keyMap[address]
	if !ok {
		ret, err := c.client.InvokeBVMContract(constant.AppchainMgrContractAddr.Address(), "GetPubKeyByChainID", nil, rpcx.String(address))
		if err != nil {
			return nil, err
		}
		pubKeyBase64 := ret.Ret
		pubKey, err = base64.StdEncoding.DecodeString(string(pubKeyBase64))
		if err != nil {
			return nil, err
		}
		c.keyMap[address] = pubKey
	}

	ke, err := ecdh.NewEllipticECDH(ecdsa.S256())
	if err != nil {
		return nil, err
	}
	secret, err := ke.ComputeSecret(c.privKey, pubKey)
	if err != nil {
		return nil, err
	}
	return sym.GenerateSymKey(crypto.ThirdDES, secret)
}
