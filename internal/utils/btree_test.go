package utils

import (
	"fmt"
	"testing"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/stretchr/testify/require"
)

func TestPoolSize(t *testing.T) {
	pool := NewPool(2)
	for i := 0; i < 500; i++ {
		pool.Ibtps.ReplaceOrInsert(&MyTree{Ibtp: &pb.IBTP{Index: uint64(i)}, Index: uint64(i)})
	}
	fmt.Println(pool.Ibtps.Max())
}

func TestLess(t *testing.T) {
	tree := &MyTree{
		Index: 3,
	}
	item := &MyTree{
		Index: 4,
	}
	ok := tree.Less(item)
	require.Equal(t, true, ok)
}
