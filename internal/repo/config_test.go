package repo

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

const pierFile = "pier.toml"

func TestUnmarshalConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	rootPath := path.Join(root, pierFile)
	defer os.RemoveAll(root)

	_, err = UnmarshalConfig(root)
	require.NotNil(t, err)
	err = cp.Copy(filepath.Join("testData", pierFile), rootPath)
	require.Nil(t, err)
	config, err := UnmarshalConfig(root)
	require.Nil(t, err)
	require.Equal(t, 4, len(config.Mode.Relay.Addrs))
	require.Equal(t, 64, config.Mode.Relay.GrpcPoolSize)
}
