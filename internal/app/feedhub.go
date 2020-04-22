package app

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"

	"github.com/sirupsen/logrus"
)

// Start starts three main components of pier app
func (pier *Pier) Start() error {
	logger.WithFields(logrus.Fields{
		"id":                     pier.meta.ID,
		"interchain_counter":     pier.meta.InterchainCounter,
		"receipt_counter":        pier.meta.ReceiptCounter,
		"source_receipt_counter": pier.meta.SourceReceiptCounter,
	}).Info("Pier information")

	if err := pier.monitor.Start(); err != nil {
		return fmt.Errorf("monitor start: %w", err)
	}

	if err := pier.exec.Start(); err != nil {
		return fmt.Errorf("executor start: %w", err)
	}

	if err := pier.stub.Start(); err != nil {
		return fmt.Errorf("sync start: %w", err)
	}

	return nil
}

func (pier *Pier) start() error {
	go pier.listenEvent()

	for {
		select {
		case outPacket := <-pier.monitor.FetchIBTP():
			pier.stub.SendIBTP(outPacket)
		case inPacket := <-pier.stub.RecvIBTP():
			pier.exec.HandleIBTP(inPacket)

		}
	}
}

func (pier *Pier) listenEvent() {
	receiptCh := make(chan *pb.IBTP)
	receiptSub := pier.exec.SubscribeReceipt(receiptCh)

	defer receiptSub.Unsubscribe()

	for {
		select {
		case receipt := <-receiptCh:
			pier.stub.SendIBTP(receipt)
		case <-pier.ctx.Done():
			return
		}
	}
}
