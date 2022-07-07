package txcrypto

import (
	"encoding/base64"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/crypto/ecdh"
	"github.com/meshplus/bitxhub-kit/crypto/sym"
	"github.com/meshplus/bitxhub-model/constant"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/sirupsen/logrus"
)

type RelayCryptor struct {
	client  rpcx.Client
	privKey crypto.PrivateKey
	keyMap  map[string][]byte
	logger  logrus.FieldLogger
}

func NewRelayCryptor(client rpcx.Client, privKey crypto.PrivateKey, logger logrus.FieldLogger) (Cryptor, error) {
	keyMap := make(map[string][]byte)

	return &RelayCryptor{
		client:  client,
		privKey: privKey,
		keyMap:  keyMap,
		logger:  logger,
	}, nil
}

func (c *RelayCryptor) Encrypt(content []byte, address string) ([]byte, error) {
	des, err := c.getDesKey(address)
	if err != nil {
		return nil, err
	}
	encodeContent := base64.StdEncoding.EncodeToString(content)
	return des.Encrypt([]byte(encodeContent))
}

func (c *RelayCryptor) Decrypt(content []byte, address string) ([]byte, error) {
	des, err := c.getDesKey(address)
	if err != nil {
		return nil, err
	}
	encodeContent, err := des.Decrypt(content)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(string(encodeContent))
}

func (c *RelayCryptor) getDesKey(address string) (crypto.SymmetricKey, error) {
	pubKey, ok := c.keyMap[address]
	if !ok {
		ret, err := c.client.InvokeBVMContract(constant.AppchainMgrContractAddr.Address(), "GetPubKeyByChainID", nil, rpcx.String(address))
		if err != nil {
			c.logger.Errorln(err)
			return nil, err
		}
		key, err := base64.StdEncoding.DecodeString(string(ret.Ret))
		if err != nil {
			c.logger.Errorln(err)
			c.logger.Errorln(ret.Ret)
			return nil, err
		}
		c.keyMap[address] = key
		pubKey = key
	}
	ke, err := ecdh.NewEllipticECDH(ecdsa.S256())
	if err != nil {
		c.logger.Errorln(err)
		return nil, err
	}
	secret, err := ke.ComputeSecret(c.privKey, pubKey)
	if err != nil {
		c.logger.Errorln(err)
		return nil, err
	}
	return sym.GenerateSymKey(crypto.ThirdDES, secret)
}
