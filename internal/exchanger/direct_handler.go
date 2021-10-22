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
	ibtps *sync.Map
	ch    chan *model.WrappedIBTP
}

func NewPool() *Pool {
	return &Pool{
		ibtps: &sync.Map{},
		ch:    make(chan *model.WrappedIBTP, 40960),
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
}

func (pool *Pool) get(index uint64) *model.WrappedIBTP {
	ibtp, ok := pool.ibtps.Load(index)
	if !ok {
		return nil
	}

	return ibtp.(*model.WrappedIBTP)
}

func (ex *Exchanger) feedIBTP(wIbtp *model.WrappedIBTP) {
	act, loaded := ex.ibtps.LoadOrStore(wIbtp.Ibtp.From, NewPool())
	pool := act.(*Pool)
	pool.feed(wIbtp)

	if !loaded {
		go func(pool *Pool, wIbtp *model.WrappedIBTP) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()
			inMeta := ex.exec.QueryInterchainMeta()
			for wIbtp := range pool.ch {
				idx := inMeta[wIbtp.Ibtp.From]
				ex.logger.Infof("get ibtp %s from pool,  exp idx %d", wIbtp.Ibtp.ID(), idx+1)
				if wIbtp.Ibtp.Index <= idx {
					pool.delete(wIbtp.Ibtp.Index)
					ex.logger.Warnf("ignore ibtp with invalid index: %d", wIbtp.Ibtp.Index)
					continue
				}
				if idx+1 == wIbtp.Ibtp.Index {
					ex.processIBTP(wIbtp, pool)
					index := wIbtp.Ibtp.Index + 1
					inMeta[wIbtp.Ibtp.From] += 1
					wIbtp := pool.get(index)
					for wIbtp != nil {
						ex.processIBTP(wIbtp, pool)
						inMeta[wIbtp.Ibtp.From] += 1
						index++
						wIbtp = pool.get(index)
					}
					idx = index - 1
				} else {
					pool.put(wIbtp)
				}

			}
		}(pool, wIbtp)
	}
}

func (ex *Exchanger) processIBTP(wIbtp *model.WrappedIBTP, pool *Pool) {
	receipt, err := ex.exec.ExecuteIBTP(wIbtp)
	if err != nil {
		ex.logger.Errorf("Execute ibtp error: %s", err.Error())
		return
	}
	ex.postHandleIBTP(wIbtp.Ibtp.From, receipt)
	ex.sendIBTPCounter.Inc()
	pool.delete(wIbtp.Ibtp.Index)
}

func (ex *Exchanger) feedReceipt(ibtp *pb.IBTP) {
	act, loaded := ex.receipts.LoadOrStore(ibtp.To, NewPool())
	pool := act.(*Pool)
	pool.feed(&model.WrappedIBTP{Ibtp: ibtp, IsValid: true})

	if !loaded {
		go func(pool *Pool, ibtp *pb.IBTP) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()
			callbackMeta := ex.exec.QueryCallbackMeta()
			for wIbtp := range pool.ch {
				idx := callbackMeta[ibtp.To]
				ex.logger.Infof("get ibtp receipt %s from pool,  exp idx %d", wIbtp.Ibtp.ID(), idx+1)
				if wIbtp.Ibtp.Index <= idx {
					pool.delete(wIbtp.Ibtp.Index)
					ex.logger.Warn("ignore ibtp with invalid index")
					continue
				}
				if idx+1 == wIbtp.Ibtp.Index {
					ex.processIBTP(wIbtp, pool)
					index := ibtp.Index + 1
					callbackMeta[ibtp.To] += 1
					wIbtp := pool.get(index)
					for wIbtp != nil {
						receipt, _ := ex.exec.ExecuteIBTP(wIbtp)
						ex.postHandleIBTP(wIbtp.Ibtp.From, receipt)
						pool.delete(wIbtp.Ibtp.Index)
						index++
						callbackMeta[ibtp.To] += 1
						wIbtp = pool.get(index)
					}
					idx = index - 1
				} else {
					pool.put(wIbtp)
				}
			}
		}(pool, ibtp)
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
