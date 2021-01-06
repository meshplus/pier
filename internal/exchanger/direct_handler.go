package exchanger

import (
	"fmt"
	"sync"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

type Pool struct {
	ibtps *sync.Map
	ch    chan *pb.IBTP
}

func NewPool() *Pool {
	return &Pool{
		ibtps: &sync.Map{},
		ch:    make(chan *pb.IBTP, 40960),
	}
}


func (pool *Pool) feed(ibtp *pb.IBTP) {
	pool.ch <- ibtp
}

func (pool *Pool) put(ibtp *pb.IBTP) {
	pool.ibtps.Store(ibtp.Index, ibtp)
}

func (pool *Pool) delete(idx uint64) {
	pool.ibtps.Delete(idx)
}

func (pool *Pool) get(index uint64) *pb.IBTP {
	ibtp, ok := pool.ibtps.Load(index)
	if !ok {
		return nil
	}

	return ibtp.(*pb.IBTP)
}


func (ex *Exchanger) feedIBTP(ibtp *pb.IBTP) {
	var pool *Pool
	act, loaded := ex.ibtps.Load(ibtp.From)
	if !loaded {
		pool = NewPool()
		ex.ibtps.Store(ibtp.From, pool)
	} else {
		pool = act.(*Pool)
	}
	pool.feed(ibtp)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					logger.Error(fmt.Errorf("%v", e))
				}
			}()
			inMeta := ex.exec.QueryMeta()
			for ibtp := range pool.ch {
				idx := inMeta[ibtp.From]
				if ibtp.Index <= idx {
					pool.delete(ibtp.Index)
					logger.Warn("ignore ibtp with invalid index:{}", ibtp.Index)
					continue
				}
				if idx+1 == ibtp.Index {
					ex.processIBTP(ibtp)
					pool.delete(ibtp.Index)
					index := ibtp.Index + 1
					ibtp := pool.get(index)
					for ibtp != nil {
						ex.processIBTP(ibtp)
						pool.delete(ibtp.Index)
						index++
						ibtp = pool.get(index)
					}
				} else {
					pool.put(ibtp)
				}
			}
		}(pool)
	}
}

func (ex *Exchanger) processIBTP(ibtp *pb.IBTP) {
	receipt, err := ex.exec.ExecuteIBTP(ibtp)
	if err != nil {
		logger.Errorf("execute ibtp error:%v", err)
		return
	}
	ex.postHandleIBTP(ibtp.From, receipt)
	ex.sendIBTPCounter.Inc()
}

func (ex *Exchanger) feedReceipt(receipt *pb.IBTP) {
	var pool *Pool
	act, loaded := ex.ibtps.Load(receipt.To)
	if !loaded {
		pool = NewPool()
		ex.ibtps.Store(receipt.To, pool)
	} else {
		pool = act.(*Pool)
	}
	pool.feed(receipt)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					logger.Error(fmt.Errorf("%v", e))
				}
			}()
			callbackMeta := ex.exec.QueryMeta()
			for ibtp := range pool.ch {
				if ibtp.Index <= callbackMeta[ibtp.To]  {
					pool.delete(ibtp.Index)
					logger.Warn("ignore ibtp with invalid index")
					continue
				}
				if callbackMeta[ibtp.To]+1 == ibtp.Index {
					ex.processIBTP(ibtp)
					pool.delete(ibtp.Index)
					index := ibtp.Index + 1
					ibtp := pool.get(index)
					for ibtp != nil {
						receipt, _ := ex.exec.ExecuteIBTP(ibtp)
						ex.postHandleIBTP(ibtp.From, receipt)
						pool.delete(ibtp.Index)
						index++
						ibtp = pool.get(index)
					}
				} else {
					pool.put(ibtp)
				}
			}
		}(pool)
	}
}


func (ex *Exchanger) analysisDirectTPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	current := time.Now()
	counter := ex.sendIBTPCounter.Load()
	for {
		select {
		case <-ticker.C:
			tps := ex.sendIBTPCounter.Load() - counter
			counter = ex.sendIBTPCounter.Load()
			totalTimer := ex.sendIBTPTimer.Load()

			if tps != 0 {
				logger.WithFields(logrus.Fields{
					"tps":      tps,
					"tps_sum":  counter,
					"tps_time": totalTimer.Milliseconds() / int64(counter),
					"tps_avg":  float64(counter) / time.Since(current).Seconds(),
				}).Info("analysis")
			}

		case <-ex.ctx.Done():
			return
		}
	}
}
