package utils

import (
	"fmt"
	"testing"

	"github.com/meshplus/bitxhub-model/pb"
)

func TestName(t *testing.T) {
	pool := NewPool(2)
	for i := 0; i < 500; i++ {
		pool.Ibtps.ReplaceOrInsert(&MyTree{Ibtp: &pb.IBTP{Index: uint64(i)}, Index: uint64(i)})
	}
	fmt.Println(pool.Ibtps.Max())
}
