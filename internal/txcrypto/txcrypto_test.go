package txcrypto

import (
	"fmt"
	"testing"

	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/stretchr/testify/require"
)

func TestRelayCryptor(t *testing.T) {
	privKey1, err := asym.GenerateKey(asym.ECDSASecp256r1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKey(asym.ECDSASecp256r1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()
	require.Nil(t, err)
	addr2 := address2.String()
	keyMap1 := make(map[string][]byte)
	keyMap1[addr2] = pubBytes2
	rc1 := &RelayCryptor{
		privKey: privKey1,
		keyMap:  keyMap1,
	}

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
	privKey1, err := asym.GenerateKey(asym.ECDSASecp256r1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKey(asym.ECDSASecp256r1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()
	require.Nil(t, err)
	addr2 := address2.String()
	keyMap1 := make(map[string][]byte)
	keyMap1[addr2] = pubBytes2
	rc1 := &DirectCryptor{
		privKey: privKey1,
		keyMap:  keyMap1,
	}

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
