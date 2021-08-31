package repo

import (
	"fmt"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_loadNetworkConfig(t *testing.T) {
	// TODO
	path := "./testdata"
	nodePriivKey, err := LoadNodePrivateKey(path)
	require.Nil(t, err)
	libp2pPrivKey, err := convertToLibp2pPrivKey(nodePriivKey)
	require.Nil(t, err)
	networkConfig, err := LoadNetworkConfig(path, libp2pPrivKey)
	require.Nil(t, err)
	require.Equal(t, 4, len(networkConfig.Piers))

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
