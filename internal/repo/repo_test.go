package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	root, err := ioutil.TempDir("", "TestRepo")
	require.Nil(t, err)
	defer os.RemoveAll(root)

	err = Initialize(root)
	require.Nil(t, err)

	_, err = LoadPrivateKey(root)
	require.Nil(t, err)

	_, err = LoadNodePrivateKey(root)
	require.Nil(t, err)

	config, err := UnmarshalConfig(root)
	require.Nil(t, err)

	validators := config.Mode.Relay.GetValidators()
	require.Equal(t, 4, len(validators))

	pathRoot, err := PathRoot()
	require.Nil(t, err)
	homeRoot, err := homedir.Expand(DefaultPathRoot)
	require.Nil(t, err)
	require.Equal(t, homeRoot, pathRoot)

	rootWithDefault, err := PathRootWithDefault(root)
	require.Nil(t, err)
	require.Equal(t, root, rootWithDefault)

	err = InitConfig(filepath.Join(root, "pier.toml"))
	require.Nil(t, err)

	keyPath := KeyPath(root)
	require.Equal(t, filepath.Join(root, KeyName), keyPath)

	nodeKeyPath := NodeKeyPath(root)
	require.Equal(t, filepath.Join(root, NodeKeyName), nodeKeyPath)

	_, err = GetAPI(root)
	require.Nil(t, err)

	_, err = PluginPath()
	require.Nil(t, err)
}
