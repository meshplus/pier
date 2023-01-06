package utils

import (
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/meshplus/bitxhub-model/pb"
)

const (
	RelayDegree  = 2
	DirectDegree = 4
)

type Pool struct {
	Ibtps        *btree.BTree
	Lock         *sync.Mutex
	Time         time.Time
	CurrentIndex uint64
}

type MyTree struct {
	Ibtp  *pb.IBTP
	Index uint64
}

func (m *MyTree) Less(item btree.Item) bool {
	return m.Index < (item.(*MyTree)).Index
}

func NewPool(degree int) *Pool {
	return &Pool{
		Ibtps: btree.New(degree),
		Lock:  &sync.Mutex{},
		Time:  time.Now(),
	}
}

func (p *Pool) Release() {
	p.Ibtps.Clear(false)
}
