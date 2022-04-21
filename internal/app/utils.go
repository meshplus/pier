package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/utils"
)

func getInterchainMeta(client rpcx.Client, appchainID string) (*pb.Interchain, error) {
	ret := &pb.Interchain{}
	logger := loggers.Logger(loggers.App)

	if err := retry.Retry(func(attempt uint) error {
		tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
			"Interchain", rpcx.String(appchainID))
		if err != nil {
			return err
		}
		tx.Nonce = 1
		receipt, err := client.SendView(tx)
		if err != nil {
			logger.Errorf("Send view to get interchain meta error: %v ... retry later", err)
			return err
		} else {
			if !receipt.IsSuccess() {
				logger.Errorf("Get interchain meta receipt: %s ... retry later", string(receipt.Ret))
				return fmt.Errorf("get interchain meta receipt: %s", receipt.Ret)
			}

			if err := ret.Unmarshal(receipt.Ret); err != nil {
				logger.Errorf("Unmarshal interchain meta from bitxhub: %s ... retry later", string(receipt.Ret))
				return fmt.Errorf("unmarshal interchain meta from bitxhub: %w", err)
			}

			return nil
		}
	}, strategy.Wait(time.Second*5),
	); err != nil {
		logger.Errorf("Get interchain meta retry error: %w", err)
		return nil, fmt.Errorf("Get interchain meta retry error: %w", err)
	}

	return ret, nil
}

func getInterchainMeta2(peerMgr peermgr.PeerManager, appchainID string) (*pb.Interchain, error) {
	ret := &pb.Interchain{}
	logger := loggers.Logger(loggers.App)
	priv, err := asym.GenerateKeyPair(crypto.Secp256k1)
	if err != nil {
		return nil, err
	}
	if err := retry.Retry(func(attempt uint) error {
		tx, err := utils.GenerateContractTx(priv, pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
			"Interchain", rpcx.String(appchainID))
		if err != nil {
			return err
		}
		tx.Nonce = 1
		err = tx.Sign(priv)
		if err != nil {
			return fmt.Errorf("%w: for reason %s", utils.ErrSignTx, err.Error())
		}
		logger.Infof("did:%s", appchainID)
		receipt, err := utils.SenView(peerMgr, tx)
		if err != nil {
			logger.Errorf("Send view to get interchain meta error: %v ... retry later", err)
			return err
		} else {
			if !receipt.IsSuccess() {
				logger.Errorf("Get interchain meta receipt: %s ... retry later", string(receipt.Ret))
				return fmt.Errorf("get interchain meta receipt: %s", receipt.Ret)
			}

			if err := ret.Unmarshal(receipt.Ret); err != nil {
				logger.Errorf("Unmarshal interchain meta from bitxhub: %s ... retry later", string(receipt.Ret))
				return fmt.Errorf("unmarshal interchain meta from bitxhub: %w", err)
			}

			return nil
		}
	}, strategy.Wait(time.Second*5),
	); err != nil {
		logger.Errorf("Get interchain meta retry error: %w", err)
		return nil, fmt.Errorf("Get interchain meta retry error: %w", err)
	}

	return ret, nil
}

func getAppchainInfo(client rpcx.Client) (*appchainmgr.Appchain, error) {
	tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), "Appchain")
	if err != nil {
		return nil, err
	}
	tx.Nonce = 1
	receipt, err := client.SendView(tx)
	if err != nil {
		return nil, err
	}

	appchain := &appchainmgr.Appchain{}
	if receipt.Status == pb.Receipt_FAILED {
		return nil, fmt.Errorf("receipt: %s", receipt.Ret)
	}

	if err := json.Unmarshal(receipt.Ret, appchain); err != nil {
		return nil, fmt.Errorf("unmarshal appchain from bitxhub: %w", err)
	}

	return appchain, nil
}
