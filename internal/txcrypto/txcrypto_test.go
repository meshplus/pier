package txcrypto

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-core/appchain-mgr/mock_appchainMgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/stretchr/testify/require"
)

func TestRelayCryptor(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)

	privKey1, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()
	require.Nil(t, err)
	addr2 := address2.String()
	ret := &pb.Receipt{
		Ret: pubBytes2,
	}
	mockClient.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(ret, nil)
	rc1, err := NewRelayCryptor(mockClient, privKey1)
	require.Nil(t, err)

	address1, err := privKey1.PublicKey().Address()
	require.Nil(t, err)
	pubBytes1, err := privKey1.PublicKey().Bytes()
	require.Nil(t, err)
	addr1 := address1.String()
	keyMap2 := make(map[string][]byte)
	keyMap2[addr1] = pubBytes1
	rc2 := &RelayCryptor{
		privKey: privKey2,
		keyMap:  keyMap2,
	}

	content := []byte("bitxhub cryptor test")
	encryptBytes, err := rc1.Encrypt(content, addr2)
	require.Nil(t, err)

	decryptBytes, err := rc2.Decrypt(encryptBytes, addr1)
	require.Nil(t, err)
	require.Equal(t, decryptBytes, content)

	fmt.Println(string(decryptBytes))
}

func TestDirectCryptor(t *testing.T) {
	c := gomock.NewController(t)
	mockAppchainMgr := mock_appchainMgr.NewMockAppchainMgr(c)

	privKey1, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()
	mockAppchainMgr.EXPECT().GetPubKeyByChainID(gomock.Any()).Return(true, pubBytes2)

	require.Nil(t, err)
	addr2 := address2.String()
	mgr := &appchain.Manager{
		Mgr: mockAppchainMgr,
	}
	rc1, err := NewDirectCryptor(mgr, privKey1)
	require.Nil(t, err)

	address1, err := privKey1.PublicKey().Address()
	require.Nil(t, err)
	pubBytes1, err := privKey1.PublicKey().Bytes()
	require.Nil(t, err)
	addr1 := address1.String()
	keyMap2 := make(map[string][]byte)
	keyMap2[addr1] = pubBytes1
	rc2 := &DirectCryptor{
		privKey: privKey2,
		keyMap:  keyMap2,
	}

	content := []byte("bitxhub cryptor test")
	encryptBytes, err := rc1.Encrypt(content, addr2)
	require.Nil(t, err)

	decryptBytes, err := rc2.Decrypt(encryptBytes, addr1)
	require.Nil(t, err)
	require.Equal(t, decryptBytes, content)

	fmt.Println(string(decryptBytes))
}
