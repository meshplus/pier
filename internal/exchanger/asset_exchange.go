package exchanger

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
				// todo(tyx): handle sending error
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
			ex.logger.Info("Receive lock event from monitor")
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on lock event")
				return
			}
			if err := ex.syncer.SendLockEvent(lockEvent); err != nil {
				// todo(tyx): handle sending error
				ex.logger.Errorf("Send lock event error: %s", err.Error())
				return
			}
			ex.logger.Info("Lock event successfully")
		}
	}
}

func (ex *Exchanger) listenUnescrowEventFromSyncer() {
	ch := ex.syncer.ListenUnescrow()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			// todo(tyx): handle burn event
		}
	}
}
