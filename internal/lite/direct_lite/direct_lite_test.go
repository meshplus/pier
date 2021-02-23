package direct_lite

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMockLite(t *testing.T) {
	lite := MockLite{}

	require.Nil(t, lite.Start())
	require.Nil(t, lite.Stop())

	_, err := lite.QueryHeader(uint64(1))
	require.Nil(t, err)
}
