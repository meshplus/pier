package txcrypto

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"

	"github.com/btcsuite/btcd/btcec"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/ecdh"
	"github.com/meshplus/bitxhub-kit/crypto/sym"
)

type DirectCryptor struct {
	peerMgr peermgr.PeerManager
	privKey crypto.PrivateKey
	keyMap  map[string][]byte
}

func NewDirectCryptor(peerMgr peermgr.PeerManager, privKey crypto.PrivateKey) (Cryptor, error) {
	keyMap := make(map[string][]byte)

	return &DirectCryptor{
		peerMgr: peerMgr,
		privKey: privKey,
		keyMap:  keyMap,
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

func (d *DirectCryptor) getDesKey(chainID string) (crypto.SymmetricKey, error) {
	pubKey, ok := d.keyMap[chainID]
	if !ok {
		pubKey, err := d.getPubKeyByChainID(chainID)
		if err != nil {
			return nil, fmt.Errorf("cannot find the public key of chain ID %s: %w", chainID, err)
		}
		d.keyMap[chainID] = pubKey
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

func (d *DirectCryptor) getPubKeyByChainID(chainID string) ([]byte, error) {
	msg, err := d.peerMgr.Send(chainID, &pb.Message{
		Type:    pb.Message_PUBKEY_GET,
		Data:    nil,
		Version: nil,
	})

	if err != nil {
		return nil, err
	}

	if msg.Type != pb.Message_PUBKEY_GET_ACK {
		return nil, fmt.Errorf("invalid response message type: %v, expected %v", msg.Type, pb.Message_PUBKEY_GET_ACK)
	}

	return msg.Data, nil
}
