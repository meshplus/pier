package repo

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "TestRepo")
	require.Nil(t, err)
	defer func() {
		err = os.RemoveAll(root)
		require.Nil(t, err)
	}()
	err = Initialize(root, "Secp256k1")
	require.Nil(t, err)

	_, err = UnmarshalConfig("")
	require.NotNil(t, err)

	config, err := UnmarshalConfig(root)
	require.Nil(t, err)
	require.Equal(t, 4, len(config.Mode.Relay.Addrs))
}
