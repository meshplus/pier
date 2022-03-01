package syncer

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	contracts "github.com/meshplus/bitxhub-core/eth-contracts/interchain-contracts"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/sirupsen/logrus"
)

type Client struct {
	ctx                   context.Context                    `json:"ctx,omitempty"`
	ethClient             *ethclient.Client                  `json:"eth_client,omitempty"`
	logger                logrus.FieldLogger                 `json:"logger,omitempty"`
	interchainSwapSession *contracts.InterchainSwapSession   `json:"interchain_swap_session,omitempty"`
	appchainIndex         uint64                             `json:"appchain_index,omitempty"`
	relayIndex            uint64                             `json:"relay_index,omitempty"`
	filterOptCh           chan *bind.FilterOpts              `json:"filter_opt_ch,omitempty"`
	logCh                 chan *contracts.InterchainSwapBurn `json:"log_ch,omitempty"`
	burnCh                chan *pb.UnLock                    `json:"burn_ch,omitempty"`
	pierId                string                             `json:"pier_id,omitempty"`
}

func (c *Client) QueryBurnEventByIndex(index uint64) *pb.UnLock {
	var burnCh *pb.UnLock
	height := c.GetInterchainSwapIndex2Height(index)
	currentStart := height.Uint64()
	latestHeight, err := c.ethClient.BlockNumber(c.ctx)
	if err != nil {
		c.logger.Error("get most recent height", "error", err.Error())
		return nil
	}
	if latestHeight < currentStart {
		return nil
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
	var (
		iter *contracts.InterchainSwapBurnIterator
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
		event := iter.Event
		if c.pierId != event.Pier.Hex() || index != event.RelayIndex.Uint64() {
			continue
		}
		burnCh = &pb.UnLock{
			Token:      event.AppToken.String(),
			From:       event.Burner.String(),
			Receipt:    event.Recipient.String(),
			Amount:     event.Amount.Bytes(),
			RelayIndex: event.RelayIndex.Uint64(),
			TxId:       event.Raw.TxHash.String(),
		}

	}
	return burnCh
}

func (c *Client) InterchainSwapSession() *contracts.InterchainSwapSession {
	return c.interchainSwapSession
}

func (c *Client) AppchainIndex() uint64 {
	return c.appchainIndex
}

func (c *Client) RelayIndex() uint64 {
	return c.relayIndex
}

func InitializeJsonRpcClient(pierId string, url string, grpcClient rpcx.Client, opts ...Option) (*Client, error) {
	c := &Client{}
	cfg, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}
	c.logger = cfg.logger
	etherCli, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}
	// 1.get interchainswap evm address
	receipt, err := grpcClient.InvokeBVMContract(constant.EthHeaderMgrContractAddr.Address(),
		"GetInterchainSwapAddr", nil)
	if err != nil {
		return nil, err
	}
	if !receipt.IsSuccess() {
		return nil, fmt.Errorf("invoke GetInterchainSwapAddr: %s", receipt.Ret)
	}

	interchainSwapContract, err := contracts.NewInterchainSwap(common.HexToAddress(string(receipt.Ret)), etherCli)
	if err != nil {
		return nil, err
	}
	interchainSwapSession := &contracts.InterchainSwapSession{
		Contract: interchainSwapContract,
		CallOpts: bind.CallOpts{
			Pending: false,
		},
	}
	c.pierId = pierId
	c.interchainSwapSession = interchainSwapSession
	c.relayIndex = c.GetInterchainSwapRelayIndex().Uint64()
	c.appchainIndex = c.GetInterchainSwapAppchainIndex().Uint64()
	c.filterOptCh = make(chan *bind.FilterOpts, 1024)
	c.logCh = make(chan *contracts.InterchainSwapBurn, 1024)
	c.burnCh = make(chan *pb.UnLock, 1024)
	c.ethClient = etherCli

	c.ctx = context.Background()

	return c, nil
}

func (c *Client) Start(aRelayIndex uint64) {
	go c.filterLog(aRelayIndex)
	go c.listenBurn()
}

func (c *Client) Stop() error {
	return nil
}

func (c *Client) GetInterchainSwapIndex2Height(index uint64) *big.Int {
	var (
		height *big.Int
		err    error
	)
	c.retryFunc(func(attempt uint) error {
		height, err = c.interchainSwapSession.Index2Height(common.HexToAddress(c.pierId), new(big.Int).SetUint64(index))
		if err != nil {
			return err
		}
		return nil
	})
	return height
}

func (c *Client) GetInterchainSwapRelayIndex() *big.Int {
	var (
		index *big.Int
		err   error
	)
	c.retryFunc(func(attempt uint) error {
		index, err = c.interchainSwapSession.RelayIndex(common.HexToAddress(c.pierId))
		if err != nil {
			return err
		}
		return nil
	})
	return index
}

func (c *Client) GetInterchainSwapAppchainIndex() *big.Int {
	var (
		index *big.Int
		err   error
	)
	c.retryFunc(func(attempt uint) error {
		index, err = c.interchainSwapSession.AppchainIndex(common.HexToAddress(c.pierId))
		if err != nil {
			return err
		}
		return nil
	})
	return index
}

func (c *Client) retryFunc(handle func(uint) error) error {
	return retry.Retry(func(attempt uint) error {
		if err := handle(attempt); err != nil {
			c.logger.Errorf("retry failed for reason: %s", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(500*time.Millisecond))
}
