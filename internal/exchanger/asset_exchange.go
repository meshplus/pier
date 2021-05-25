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
			// todo(tyx): handle updateMeta
			ex.logger.Info("mint event: %v", updateMeta)
		}
	}
}

func (ex *Exchanger) listenMintEvent() {
	ch := ex.mnt.ListenMintEvent()
	for {
		select {
		case <-ex.ctx.Done():
			return
		case mintEvent, ok := <-ch:
			ex.logger.Info("Receive mint event from monitor")
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on mint event")
				return
			}
			// todo(tyx): handle mintEvent
			ex.logger.Info("mint event: %v", mintEvent)
		}
	}
}
