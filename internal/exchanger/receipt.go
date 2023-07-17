package exchanger

import (
	"fmt"

	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

// 这个pool是当前应用链作为发起方，按照map[to]*pool组织的
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
					entry.Infof("receive ibtp[%s], typ: %s", ibtp.ID(), ibtp.Type)
					if ibtp.Index <= ex.callbackCounter[ibtp.To] {
						pool.delete(ibtp.Index)
						entry.Warn("ibtp.Index(%d) < ex.callbackCounter[ibtp.To](%d), "+
							"ignore ibtp with invalid index", ibtp.Index, ibtp.To, ex.callbackCounter[ibtp.To])
						continue
					}
					if ex.callbackCounter[ibtp.To]+1 == ibtp.Index {
						// if this is a failed receipt, try to rollback
						// else handle it in normal way
						entry.Infof("ibtp[%s].isValid: %v", ibtp.ID(), wIbtp.IsValid)
						if wIbtp.IsValid {
							ex.handleIBTP(wIbtp)
						} else {
							ex.exec.Rollback(ibtp, true)
						}

						entry.Infof("update exchanger.callbackCounter[%s]=%d", ibtp.To, ibtp.Index)
						ex.callbackCounter[ibtp.To] += 1
						pool.delete(ibtp.Index)
						index := ibtp.Index + 1
						wIbtp := pool.get(index)
						for wIbtp != nil {
							entry.Infof("feedIBTPReceipt continue consume sequential ibtps, with index=%d", index)
							entry.Infof("ibtp[%s].isValid: %v", ibtp.ID(), wIbtp.IsValid)
							ibtp := wIbtp.Ibtp
							if wIbtp.IsValid {
								ex.handleIBTP(wIbtp)
							} else {
								ex.exec.Rollback(ibtp, true)
							}
							entry.Infof("update exchanger.callbackCounter[%s]=%d", ibtp.To, ibtp.Index)
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
