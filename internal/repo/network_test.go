package repo

import (
	"fmt"
	"testing"

	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/stretchr/testify/require"
)

const testConfigPath = "./testdata"

func TestLoadNetworkConfig(t *testing.T) {
	nodePriKey, err := LoadNodePrivateKey(testConfigPath)
	require.Nil(t, err)
	libP2PPriKey, err := convertToLibP2PPriKey(nodePriKey)
	require.Nil(t, err)

	_, err = LoadNetworkConfig("", libP2PPriKey)
	require.NotNil(t, err)

	networkConfig, err := LoadNetworkConfig(testConfigPath, libP2PPriKey)
	require.Nil(t, err)
	require.Equal(t, 4, len(networkConfig.Piers))
}

func convertToLibP2PPriKey(privateKey crypto.PrivateKey) (crypto2.PrivKey, error) {
	ecdsaPriKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("convert to libp2p private key: not ecdsa private key")
	}

	libP2PPriKey, _, err := crypto2.ECDSAKeyPairFromKey(ecdsaPriKey.K)
	if err != nil {
		return nil, err
	}

	return libP2PPriKey, nil
}

func TestGetNetworkPeers(t *testing.T) {
	path := "./testdata"
	nodePriKey, err := LoadNodePrivateKey(path)
	require.Nil(t, err)
	libP2PPriKey, err := convertToLibP2PPriKey(nodePriKey)
	require.Nil(t, err)
	networkConfig, err := LoadNetworkConfig(path, libP2PPriKey)
	require.Nil(t, err)

	peers, err := GetNetworkPeers(networkConfig)
	require.Nil(t, err)
	require.Equal(t, "QmfMVpeCzcMgvsVLojteBf5YxWyPfHkjV67bPz1gUAFX1B",
		peers["QmfMVpeCzcMgvsVLojteBf5YxWyPfHkjV67bPz1gUAFX1B"].ID.String())
	require.Equal(t, "/ip4/127.0.0.1/tcp/4321",
		peers["QmfMVpeCzcMgvsVLojteBf5YxWyPfHkjV67bPz1gUAFX1B"].Addrs[0].String())
}

func TestWriteNetworkConfig(t *testing.T) {
	nodePriKey, err := LoadNodePrivateKey(testConfigPath)
	require.Nil(t, err)
	libP2PPriKey, err := convertToLibP2PPriKey(nodePriKey)
	require.Nil(t, err)
	networkConfig, err := LoadNetworkConfig(testConfigPath, libP2PPriKey)
	require.Nil(t, err)

	err = WriteNetworkConfig("", testConfigPath, networkConfig)
	require.Nil(t, err)

	err = WriteNetworkConfig(testConfigPath, testConfigPath, networkConfig)
	require.Nil(t, err)

	err = WriteNetworkConfig("t", testConfigPath, networkConfig)
	require.NotNil(t, err)

	err = WriteNetworkConfig("", "", networkConfig)
	require.NotNil(t, err)
}
