package exchanger

import (
	"fmt"

	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) feedIBTPReceipt(receipt *model.WrappedIBTP) {
	var pool *Pool
	act, loaded := ex.receipts.Load(receipt.Ibtp.To)
	if !loaded {
		pool = NewPool()
		ex.receipts.Store(receipt.Ibtp.To, pool)
	} else {
		pool = act.(*Pool)
	}
	pool.feed(receipt)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()
			for wIbtp := range pool.ch {
				entry := ex.logger.WithFields(logrus.Fields{"type": wIbtp.Ibtp.Type, "id": wIbtp.Ibtp.ID()})
				ibtp := wIbtp.Ibtp
				if ibtp.Index <= ex.callbackCounter[ibtp.To] {
					pool.delete(ibtp.Index)
					entry.Warn("Ignore ibtp with invalid index")
					continue
				}
				if pool.has(ibtp.Index) {
					// ignore duplicated receipt
					entry.Warn("Ignore ibtp with duplicated index")
					continue
				}
				if ex.callbackCounter[ibtp.To]+1 == ibtp.Index {
					// if this is a failed receipt, try to rollback
					// else handle it in normal way
					if wIbtp.IsValid {
						ex.handleIBTP(wIbtp, entry)
					} else {
						ex.exec.Rollback(ibtp, true)
					}

					ex.callbackCounter[ibtp.To] += 1
					pool.delete(ibtp.Index)
					index := ibtp.Index + 1
					wIbtp := pool.get(index)
					for wIbtp != nil {
						ibtp := wIbtp.Ibtp
						if wIbtp.IsValid {
							ex.handleIBTP(wIbtp, entry)
						} else {
							ex.exec.Rollback(ibtp, true)
						}
						ex.callbackCounter[ibtp.To] += 1
						pool.delete(ibtp.Index)
						index++
						wIbtp = pool.get(index)
					}
				} else {
					if wIbtp != nil {
						pool.put(wIbtp)
					}
				}
			}
		}(pool)
	}
}
