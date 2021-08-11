package exchanger

import (
	"fmt"
	"sync"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

type Pool struct {
	ibtps    *sync.Map
	ch       chan *model.WrappedIBTP
	beginIdx uint64
}

func NewPool(beginIdx uint64) *Pool {
	return &Pool{
		ibtps:    &sync.Map{},
		ch:       make(chan *model.WrappedIBTP, 40960),
		beginIdx: beginIdx,
	}
}

func (pool *Pool) feed(ibtp *model.WrappedIBTP) {
	pool.ch <- ibtp
}

func (pool *Pool) put(ibtp *model.WrappedIBTP) {
	pool.ibtps.Store(ibtp.Ibtp.Index, ibtp)
}

func (pool *Pool) delete(idx uint64) {
	pool.ibtps.Delete(idx)
	if idx == pool.beginIdx {
		pool.beginIdx++
	}
}

func (pool *Pool) get(index uint64) *model.WrappedIBTP {
	ibtp, ok := pool.ibtps.Load(index)
	if !ok {
		return nil
	}

	return ibtp.(*model.WrappedIBTP)
}

func (ex *Exchanger) feedIBTP(wIbtp *model.WrappedIBTP) {
	var pool *Pool
	ibtp := wIbtp.Ibtp
	act, loaded := ex.ibtps.Load(ibtp.From)
	if !loaded {
		pool = NewPool(ex.inMeta[ibtp.ServicePair()] + 1)
		ex.ibtps.Store(ibtp.From, pool)
	} else {
		pool = act.(*Pool)
	}
	pool.feed(wIbtp)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()
			for wIbtp := range pool.ch {
				ibtp := wIbtp.Ibtp
				if ibtp.Index < pool.beginIdx {
					pool.delete(ibtp.Index)
					ex.logger.Warnf("ignore ibtp with invalid index: %d", ibtp.Index)
					continue
				}
				if pool.beginIdx == ibtp.Index {
					ex.processIBTP(wIbtp)
					pool.delete(ibtp.Index)
					index := ibtp.Index + 1
					wIbtp := pool.get(index)
					for wIbtp != nil {
						ex.processIBTP(wIbtp)
						pool.delete(wIbtp.Ibtp.Index)
						index++
						wIbtp = pool.get(index)
					}
				} else {
					pool.put(wIbtp)
				}
			}
		}(pool)
	}
}

func (ex *Exchanger) processIBTP(wIbtp *model.WrappedIBTP) {
	receipt, err := ex.exec.ExecuteIBTP(wIbtp)
	if err != nil {
		ex.logger.Errorf("Execute ibtp error: %s", err.Error())
		return
	}
	ex.postHandleIBTP(wIbtp.Ibtp.From, receipt)
	ex.sendIBTPCounter.Inc()
}

func (ex *Exchanger) feedReceipt(receipt *pb.IBTP) {
	var pool *Pool
	act, loaded := ex.ibtps.Load(receipt.To)
	if !loaded {
		pool = NewPool(ex.callbackMeta[receipt.ServicePair()] + 1)
		ex.ibtps.Store(receipt.To, pool)
	} else {
		pool = act.(*Pool)
	}
	pool.feed(&model.WrappedIBTP{Ibtp: receipt, IsValid: true})

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()
			for wIbtp := range pool.ch {
				ibtp := wIbtp.Ibtp
				if ibtp.Index < pool.beginIdx {
					pool.delete(ibtp.Index)
					ex.logger.Warn("ignore ibtp with invalid index")
					continue
				}
				if pool.beginIdx == ibtp.Index {
					ex.processIBTP(wIbtp)
					pool.delete(ibtp.Index)
					index := ibtp.Index + 1
					wIbtp := pool.get(index)
					for wIbtp != nil {
						ibtp := wIbtp.Ibtp
						receipt, _ := ex.exec.ExecuteIBTP(wIbtp)
						ex.postHandleIBTP(ibtp.From, receipt)
						pool.delete(ibtp.Index)
						index++
						wIbtp = pool.get(index)
					}
				} else {
					pool.put(wIbtp)
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
				ex.logger.WithFields(logrus.Fields{
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
