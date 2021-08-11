package syncer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/loggers"
)

func GetServiceInterchainMeta(client rpcx.Client, bxhID, appchainID string) (map[string]*pb.Interchain, error) {
	var (
		services           []string
		serviceInterchains = make(map[string]*pb.Interchain)
	)
	tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.ServiceMgrContractAddr.Address(),
		"GetServicesByAppchainID", rpcx.String(appchainID))
	if err != nil {
		return nil, err
	}

	ret := getTxView(client, tx)

	if err := json.Unmarshal(ret, &services); err != nil {
		return nil, err
	}

	for _, service := range services {
		fullServiceID := fmt.Sprintf("%s:%s:%s", bxhID, appchainID, service)
		serviceInterchains[fullServiceID] = GetInterchainByServiceID(client, fullServiceID)
	}

	return serviceInterchains, nil
}

func GetInterchainByServiceID(client rpcx.Client, fullServiceID string) *pb.Interchain {
	tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetInterchain", rpcx.String(fullServiceID))
	if err != nil {
		panic(err)
	}

	ret := getTxView(client, tx)

	interchain := &pb.Interchain{}
	if err := interchain.Unmarshal(ret); err != nil {
		panic(err)
	}

	return interchain
}

func getTxView(client rpcx.Client, tx *pb.BxhTransaction) []byte {
	var (
		receipt *pb.Receipt
		err     error
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
				logger.Errorf("get tx view receipt: %s ... retry later", string(receipt.Ret))
				return fmt.Errorf("get tx view receipt: %s", string(receipt.Ret))
			}

			return nil
		}
	}, strategy.Wait(time.Second*5)); err != nil {
		logger.Panicf("get tx view retry error: %v", err)
	}

	return receipt.GetRet()
}
