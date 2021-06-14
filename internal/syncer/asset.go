package syncer

import (
	"math/big"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	contracts "github.com/meshplus/bitxhub-core/eth-contracts"
	"github.com/meshplus/bitxhub-model/pb"
)

func (c *Client) listenBurn() {
	for {
		select {
		case log := <-c.logCh:
			// query this block from ethereum and generate mintEvent and proof for pier
			if err := retry.Retry(func(attempt uint) error {

				c.burnCh <- &pb.UnLock{
					Token:      log.EthToken.String(),
					From:       log.Burner.String(),
					Receipt:    log.Recipient.String(),
					Amount:     log.Amount.Uint64(),
					RelayIndex: log.RelayIndex.Uint64(),
					TxId:       log.Raw.TxHash.String(),
				}
				return nil
			}, strategy.Wait(1*time.Second)); err != nil {
				logger.Error("Can't retrieve burn event from receipt", "error", err.Error())
			}
		case filterOpt := <-c.filterOptCh:
			var (
				iter *contracts.InterchainSwapBurnIterator
				err  error
			)
			if err := retry.Retry(func(attempt uint) error {
				iter, err = c.interchainSwapSession.Contract.FilterBurn(filterOpt)
				if err != nil {
					return err
				}
				return nil
			}, strategy.Wait(1*time.Second)); err != nil {
				logger.Error("Can't get filter burn event", "error", err.Error())
			}
			for iter.Next() {
				c.logCh <- iter.Event
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) filterLog(aRelayIndex int64) {
	height, _ := c.interchainSwapSession.Index2Height(big.NewInt(aRelayIndex))
	start := height.Uint64()
	end := start + 500
	filterOpt := &bind.FilterOpts{
		Start: start,
		End:   &end,
	}
	c.filterOptCh <- filterOpt
}
