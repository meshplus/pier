package txcrypto

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/ecdh"
	"github.com/meshplus/bitxhub-kit/crypto/sym"
	"github.com/meshplus/pier/internal/appchain"
)

type DirectCryptor struct {
	appchainMgr *appchain.Manager
	privKey     crypto.PrivateKey
	keyMap      map[string][]byte
	isEncrypted bool
}

func NewDirectCryptor(appchainMgr *appchain.Manager, privKey crypto.PrivateKey, isEncrypted bool) (Cryptor, error) {
	keyMap := make(map[string][]byte)

	return &DirectCryptor{
		appchainMgr: appchainMgr,
		privKey:     privKey,
		keyMap:      keyMap,
		isEncrypted: isEncrypted,
	}, nil
}

func (d *DirectCryptor) Encrypt(content []byte, address string) ([]byte, error) {
	des, err := d.getDesKey(address)
	if err != nil {
		return nil, err
	}
	return des.Encrypt(content)
}

func (d *DirectCryptor) Decrypt(content []byte, address string) ([]byte, error) {
	des, err := d.getDesKey(address)
	if err != nil {
		return nil, err
	}
	return des.Decrypt(content)
}

func (c *DirectCryptor) IsPrivacy() bool {
	return c.isEncrypted
}

func (d *DirectCryptor) getDesKey(address string) (crypto.SymmetricKey, error) {
	pubKey, ok := d.keyMap[address]
	if !ok {
		get, ret := d.appchainMgr.Mgr.GetPubKeyByChainID(address)
		if !get {
			return nil, fmt.Errorf("cannot find the public key")
		}
		d.keyMap[address] = ret
		pubKey = ret
	}
	ke, err := ecdh.NewEllipticECDH(btcec.S256())
	if err != nil {
		return nil, err
	}
	secret, err := ke.ComputeSecret(d.privKey, pubKey)
	if err != nil {
		return nil, err
	}
	return sym.GenerateSymKey(crypto.ThirdDES, secret)
}
