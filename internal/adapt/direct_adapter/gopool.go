package direct_adapter

import (
	"sync"
)

type pool struct {
	queue chan struct{}
	wg    *sync.WaitGroup
}

func NewGoPool(size int) *pool {
	if size <= 0 {
		size = 1
	}
	return &pool{
		queue: make(chan struct{}, size),
		wg:    &sync.WaitGroup{},
	}
}

func (p *pool) Add() {
	p.queue <- struct{}{}
	p.wg.Add(1)
}

func (p *pool) Done() {
	<-p.queue
	p.wg.Done()
}

func (p *pool) Wait() {
	p.wg.Wait()
}
