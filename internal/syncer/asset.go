package syncer

import (
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	contracts "github.com/meshplus/bitxhub-core/eth-contracts/interchain-contracts"
	"github.com/meshplus/bitxhub-model/pb"
)

func (c *Client) listenBurn() {
	for {
		select {
		case log := <-c.logCh:
			// query this block from ethereum and generate mintEvent and proof for pier
			if strings.EqualFold(log.Pier.String(), c.pierId) {
				c.burnCh <- &pb.UnLock{
					Token:      log.AppToken.String(),
					From:       log.Burner.String(),
					Receipt:    log.Recipient.String(),
					Amount:     log.Amount.Bytes(),
					RelayIndex: log.RelayIndex.Uint64(),
					TxId:       log.Raw.TxHash.String(),
				}
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
				c.logger.Error("Can't get filter burn event", "error", err.Error())
			}
			for iter.Next() {
				c.logCh <- iter.Event
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) filterLog(aRelayIndex uint64) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	height := c.GetInterchainSwapIndex2Height(aRelayIndex)
	currentStart := height.Uint64()

	for {
		select {
		case <-ticker.C:
			// get latest blockchain height and got all finalized headers into pool
			latestHeight, err := c.ethClient.BlockNumber(c.ctx)
			if err != nil {
				c.logger.Error("get most recent height", "error", err.Error())
				continue
			}
			if latestHeight < currentStart {
				continue
			}
			var end uint64

			if latestHeight < currentStart+500 {
				end = latestHeight
			} else {
				end = currentStart + 500
			}
			filterOpt := &bind.FilterOpts{
				Start: currentStart,
				End:   &end,
			}

			c.filterOptCh <- filterOpt
			currentStart = end + 1
		case <-c.ctx.Done():
			ticker.Stop()
			return
		}
	}
}
