package utils

import (
	"sync"
)

type GoPool struct {
	queue chan struct{}
	wg    *sync.WaitGroup
}

func NewGoPool(size int) *GoPool {
	if size <= 0 {
		size = 1
	}
	return &GoPool{
		queue: make(chan struct{}, size),
		wg:    &sync.WaitGroup{},
	}
}

func (p *GoPool) Add() {
	p.queue <- struct{}{}
	p.wg.Add(1)
}

func (p *GoPool) Done() {
	<-p.queue
	p.wg.Done()
}

func (p *GoPool) Wait() {
	p.wg.Wait()
}
