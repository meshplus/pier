package app

import (
	"encoding/json"
	"fmt"

	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

func getInterchainMeta(client rpcx.Client) (*pb.Interchain, error) {
	tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(), "Interchain")
	if err != nil {
		return nil, err
	}
	tx.Nonce = 1
	receipt, err := client.SendView(tx)
	if err != nil {
		return nil, err
	}
	if !receipt.IsSuccess() {
		return nil, fmt.Errorf("get interchain meta receipt: %s", receipt.Ret)
	}

	ret := &pb.Interchain{}
	if err := ret.Unmarshal(receipt.Ret); err != nil {
		return nil, fmt.Errorf("unmarshal interchain meta from bitxhub: %w", err)
	}

	return ret, nil
}

func getAppchainInfo(client rpcx.Client) (*rpcx.Appchain, error) {
	tx, err := client.GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), "Appchain")
	if err != nil {
		return nil, err
	}
	tx.Nonce = 1
	receipt, err := client.SendView(tx)
	if err != nil {
		return nil, err
	}

	appchain := &rpcx.Appchain{}
	if receipt.Status == pb.Receipt_FAILED {
		return nil, fmt.Errorf("receipt: %s", receipt.Ret)
	}

	if err := json.Unmarshal(receipt.Ret, appchain); err != nil {
		return nil, fmt.Errorf("unmarshal appchain from bitxhub: %w", err)
	}

	return appchain, nil
}
