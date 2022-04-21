package bxh_lite

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

func (lite *BxhLite2) handleBlockHeader(header *pb.BlockHeader) {
	if header == nil {
		lite.logger.WithField("height", lite.height).Error("empty block header")
		return
	}

	if header.Number < lite.getDemandHeight() {
		return
	}

	if ok, err := lite.verifyHeader(header); !ok {
		lite.logger.WithFields(logrus.Fields{
			"height": header.Number,
			"error":  err,
		}).Warn("Invalid header")
		return
	}

	if err := lite.persist(header); err != nil {
		lite.logger.WithFields(logrus.Fields{
			"height": header.Number,
			"error":  err,
		}).Error("Persist block header")
	}

	lite.updateHeight()

	lite.logger.WithFields(logrus.Fields{
		"height": header.Number,
	}).Info("Persist block header")
}

func (lite *BxhLite2) syncBlock() {
	req := &pb.SubscriptionRequest{Type: pb.SubscriptionRequest_BLOCK_HEADER}
	reqData, err := req.Marshal()
	if err != nil {
		lite.logger.Errorf("SubscriptionRequest marshal err: %w", err)
	}
	msg := peermgr.Message(pb.Message_PIER_SUBSCRIBE_BLOCK_HEADER, true, reqData)
	msg.From = fmt.Sprintf("%s, %s", lite.peerMgr.GetLocalAddr(), lite.peerMgr.GetPangolinAddr())
	data, err := msg.Marshal()
	if err != nil {
		lite.logger.Errorf("lite subscrbe marshal err: %w", err)
	}
	if err = retry.Retry(func(attempt uint) error {
		resp, err := lite.peerMgr.SendByMultiAddr(data)
		if err != nil {
			lite.logger.Errorf("lite SendByMultiAddr subscrbe block header err0: %w", err)
			return err
		}
		if !strings.Contains(string(resp), peermgr.SubscribeResponse) {
			err = fmt.Errorf("bitxhub is not recieve block header subscribe request, res: %s", resp)
			lite.logger.Errorf("lite AsyncSendByMultiAddr subscrbe block header err: %w", err)
			return err
		}
		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}
	for {
		select {
		case header, ok := <-lite.headerCh:
			if !ok {
				lite.logger.Warn("Unexpected closed channel while syncing block header")
				return
			}
			lite.handleBlockHeader(header)
		case <-lite.ctx.Done():
			close(lite.headerCh)
			return
		}
	}
}

func (lite *BxhLite2) getHeaderChannel() chan *pb.BlockHeader {
	ch := make(chan *pb.BlockHeader, maxChSize)

	if err := retry.Retry(func(attempt uint) error {
		if err := lite.syncBlockHeader(ch); err != nil {
			return err
		}

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	return ch
}

func (lite *BxhLite2) syncBlockHeader(headerCh chan<- *pb.BlockHeader) error {
	req := &pb.SubscriptionRequest{Type: pb.SubscriptionRequest_BLOCK_HEADER}
	reqData, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("SubscriptionRequest marshal err: %w", err)
	}
	msg := peermgr.Message(pb.Message_PIER_SUBSCRIBE_BLOCK_HEADER, true, reqData)
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("lite subscrbe marshal err: %w", err)
	}
	resp, err := lite.peerMgr.SendByMultiAddr(data)
	if err != nil {
		return fmt.Errorf("lite SendByMultiAddr subscrbe block header err1: %w", err)
	}
	if !strings.Contains(string(resp), peermgr.SubscribeResponse) {
		err = fmt.Errorf("bitxhub is not recieve block header subscribe request, res: %s", resp)
		lite.logger.Errorf("lite SendByMultiAddr subscrbe block header err1: %w", err)
		return err
	}

	go func() {
		for {
			select {
			case <-lite.ctx.Done():
				return
			case h, ok := <-lite.headerCh:
				if !ok {
					close(headerCh)
					return
				}
				headerCh <- h
			}
		}
	}()
	return nil
}
