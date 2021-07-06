package exchanger

import (
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
)

func (ex *Exchanger) listenUpdateMeta() {
	ch := ex.mnt.ListenUpdateMeta()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case updateMeta, ok := <-ch:
			ex.logger.Info("Receive to-update meta from monitor")
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on update meta")
				return
			}
			if err := ex.syncer.SendUpdateMeta(updateMeta); err != nil {
				ex.logger.Errorf("Send update meta error: %s", err.Error())
				return
			}
			ex.logger.Info("Update meta event successfully")
		}
	}
}

func (ex *Exchanger) listenMintEvent() {
	ch := ex.mnt.ListenLockEvent()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case lockEvent, ok := <-ch:
			// todo log add event
			ex.logger.Info("Receive lock event from monitor")
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on lock event")
				return
			}
			if lockEvent.AppchainIndex <= ex.rAppchainIndex {
				continue
			}
			// do handleMissingEvent
			ex.handleMissingLockFromMnt(ex.rAppchainIndex, lockEvent.GetAppchainIndex()-1)
			if err := ex.syncer.SendLockEvent(lockEvent); err != nil {
				ex.logger.Errorf("Send lock event error: %s", err.Error())
				return
			}
			ex.rAppchainIndex++
			ex.logger.Info("Lock event successfully")
		}
	}
}

func (ex *Exchanger) listenBurnEventFromSyncer() {
	// start bxhJsonRpc client
	ex.syncer.JsonrpcClient().Start(ex.aRelayIndex)
	ch := ex.syncer.ListenBurn()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case burnEvent, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain burn event")
				return
			}
			if burnEvent.RelayIndex <= ex.aRelayIndex {
				continue
			}
			// do handleMissingEvent
			ex.handleMissingBurnFromSyncer(ex.aRelayIndex, burnEvent.GetRelayIndex()-1)
			// get mutil signs
			var (
				multiSigns [][]byte
				err        error
			)
			ex.retryFunc(func(attempt uint) error {
				multiSigns, err = ex.syncer.GetEVMSigns(burnEvent.TxId)
				if err != nil {
					return err
				}
				return nil
			})
			burnEvent.MultiSigns = multiSigns
			if err := ex.exec.SendBurnEvent(burnEvent); err != nil {
				// handle sending error
				ex.logger.Errorf("Send unlock event error: %s", err.Error())
				return
			}
			ex.aRelayIndex++
			ex.logger.Infof("unlock event successfully, txid=%s", burnEvent.TxId)
		}
	}
}

func (ex *Exchanger) retryFunc(handle func(uint) error) error {
	return retry.Retry(func(attempt uint) error {
		if err := handle(attempt); err != nil {
			ex.logger.Errorf("retry failed for reason: %s", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(500*time.Millisecond))
}
