package exchanger

import (
	"fmt"

	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

func (ex *Exchanger) feedIBTPReceipt(receipt *model.WrappedIBTP) {
	act, loaded := ex.receipts.LoadOrStore(receipt.Ibtp.To, NewPool())
	pool := act.(*Pool)
	pool.feed(receipt)

	if !loaded {
		go func(pool *Pool) {
			defer func() {
				if e := recover(); e != nil {
					ex.logger.Error(fmt.Errorf("%v", e))
				}
			}()

			for {
				select {
				case wIbtp := <-pool.ch:
					entry := ex.logger.WithFields(logrus.Fields{"type": wIbtp.Ibtp.Type, "id": wIbtp.Ibtp.ID()})
					ibtp := wIbtp.Ibtp
					if ibtp.Index <= ex.callbackCounter[ibtp.To] {
						pool.delete(ibtp.Index)
						entry.Warn("Ignore ibtp with invalid index")
						continue
					}
					if ex.callbackCounter[ibtp.To]+1 == ibtp.Index {
						// if this is a failed receipt, try to rollback
						// else handle it in normal way
						if wIbtp.IsValid {
							ex.handleIBTP(wIbtp)
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
								ex.handleIBTP(wIbtp)
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
				case <-ex.ctx.Done():
					return
				}
			}
		}(pool)
	}
}
