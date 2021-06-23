package syncer

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/hashicorp/go-hclog"
	contracts "github.com/meshplus/bitxhub-core/eth-contracts"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type Client struct {
	ctx                   context.Context
	ethClient             *ethclient.Client
	interchainSwapSession *contracts.InterchainSwapSession
	appchainIndex         int64
	relayIndex            int64
	filterOptCh           chan *bind.FilterOpts
	logCh                 chan *contracts.InterchainSwapBurn
	burnCh                chan *pb.UnLock
}

func (c *Client) QueryBurnEventByIndex(index int64) *pb.UnLock {
	var burnCh *pb.UnLock
	height, _ := c.interchainSwapSession.Index2Height(big.NewInt(index))
	end := height.Uint64()
	filterOpt := &bind.FilterOpts{
		Start: end,
		End:   &end,
	}
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
		event := iter.Event
		if index != event.RelayIndex.Int64() {
			continue
		}
		// query muti-sign from relay chain
		if err := retry.Retry(func(attempt uint) error {
			burnCh = &pb.UnLock{
				Token:      event.EthToken.String(),
				From:       event.Burner.String(),
				Receipt:    event.Recipient.String(),
				Amount:     event.Amount.Bytes(),
				RelayIndex: event.RelayIndex.Uint64(),
				TxId:       event.Raw.TxHash.String(),
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Error("Can't retrieve mint event from receipt", "error", err.Error())
		}
	}
	return burnCh
}

func (c *Client) InterchainSwapSession() *contracts.InterchainSwapSession {
	return c.interchainSwapSession
}

func (c *Client) AppchainIndex() int64 {
	return c.appchainIndex
}

func (c *Client) RelayIndex() int64 {
	return c.relayIndex
}

var (
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "client",
		Output: os.Stderr,
		Level:  hclog.Trace,
	})
)

func InitializeJsonRpcClient(url string, grpcClient rpcx.Client) (*Client, error) {

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

	c := &Client{}
	relayIndex, _ := interchainSwapSession.RelayIndex()
	c.relayIndex = relayIndex.Int64()
	appchainIndex, _ := interchainSwapSession.AppchainIndex()
	c.appchainIndex = appchainIndex.Int64()
	c.filterOptCh = make(chan *bind.FilterOpts, 1024)
	c.logCh = make(chan *contracts.InterchainSwapBurn, 1024)
	c.burnCh = make(chan *pb.UnLock, 1024)
	c.ethClient = etherCli
	c.interchainSwapSession = interchainSwapSession
	c.ctx = context.Background()

	return c, nil
}

func (c *Client) Start(aRelayIndex int64) {
	go c.filterLog(aRelayIndex)
	go c.listenBurn()
}

func (c *Client) Stop() error {
	return nil
}
