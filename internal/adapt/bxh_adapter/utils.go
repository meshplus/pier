package bxh_adapter

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/loggers"
)

const (
	CurAppchainNotAvailable    = "current appchain not available"
	TargetAppchainNotAvailable = "target appchain not available"
	SrcBitXHubNotAvailable     = "source bitxhub not available"
	TargetBitXHubNotAvailable  = "target bitxhub not available"
	CurServiceNotAvailable     = "current service not available"
	TargetServiceNotAvailable  = "target service not available"

	proofFailed          = "verify failed"
	invalidIBTP          = "invalid ibtp"
	ibtpIndexExist       = "index already exists"
	ibtpIndexWrong       = "wrong index"
	noBindRule           = "appchain didn't register rule"
	InvalidTargetService = "invalid target service"
	ibtpRollback         = "state BEGIN_ROLLBACK get unexpected receip"
)

func getTxView(client rpcx.Client, tx *pb.BxhTransaction) []byte {
	var (
		receipt     *pb.Receipt
		err         error
		emptyResult bool
	)
	tx.Nonce = 1
	logger := loggers.Logger(loggers.App)
	if err := retry.Retry(func(attempt uint) error {
		receipt, err = client.SendView(tx)
		if err != nil {
			logger.Errorf("send view to bitxhub error: %v ... retry later", err)
			return err
		} else {
			if !receipt.IsSuccess() {
				logger.Errorf("get tx view receipt: %s", string(receipt.Ret))
				if strings.Contains(string(receipt.Ret), "this service does not exist") ||
					strings.Contains(string(receipt.Ret), "interchain info does not exist") {
					emptyResult = true
					return nil
				}
				return fmt.Errorf("get tx view receipt: %s", string(receipt.Ret))
			}

			return nil
		}
	}, strategy.Wait(time.Second*5)); err != nil {
		logger.Panicf("get tx view retry error: %v", err)
	}

	if emptyResult {
		return nil
	}

	return receipt.GetRet()
}
