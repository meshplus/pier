package bxh_lite

import (
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

func (lite *BxhLite) handleBlockHeader(header *pb.BlockHeader) {
	if header == nil {
		logger.WithField("height", lite.height).Error("empty block header")
		return
	}

	if header.Number < lite.getDemandHeight() {
		return
	}

	if ok, err := lite.verifyHeader(header); !ok {
		logger.WithFields(logrus.Fields{
			"height": header.Number,
			"error":  err,
		}).Warn("Invalid header")
		return
	}

	logger.WithFields(logrus.Fields{
		"height": header.Number,
	}).Info("Persist block header")

	if err := lite.persist(header); err != nil {
		logger.WithFields(logrus.Fields{
			"height": header.Number,
			"error":  err,
		}).Error("Persist block header")
	}

	lite.updateHeight()
}

func (lite *BxhLite) syncBlock() {
	loop := func(ch chan *pb.BlockHeader) {
		for {
			select {
			case header, ok := <-ch:
				if !ok {
					logger.Warn("Unexpected closed channel while syncing block header")
					return
				}

				lite.handleBlockHeader(header)
			default:
				logger.Errorf("Not supported type for sync block")
			case <-lite.ctx.Done():
				return
			}
		}
	}

	for {
		headerCh := lite.getHeaderChannel()
		err := retry.Retry(func(attempt uint) error {
			chainMeta, err := lite.ag.GetChainMeta()
			if err != nil {
				logger.WithField("error", err).Error("Get chain meta")
				return err
			}

			if chainMeta.Height > lite.height {
				lite.recover(lite.getDemandHeight(), chainMeta.Height)
			}

			return nil
		}, strategy.Wait(1*time.Second))

		if err != nil {
			logger.Panic(err)
		}

		loop(headerCh)
	}
}

func (lite *BxhLite) getHeaderChannel() chan *pb.BlockHeader {
	ch := make(chan *pb.BlockHeader)

	if err := retry.Retry(func(attempt uint) error {
		c, err := lite.ag.SyncBlockHeader(lite.ctx)
		if err != nil {
			return err
		}

		ch = c

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	return ch
}
