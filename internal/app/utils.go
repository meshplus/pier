package app

import (
	"encoding/json"
	"fmt"

	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

func getInterchainMeta(client rpcx.Client) (*pb.Interchain, error) {
	receipt, err := client.InvokeBVMContract(constant.InterchainContractAddr.Address(), "Interchain", nil)
	if err != nil {
		return nil, err
	}

	if !receipt.IsSuccess() {
		return nil, fmt.Errorf("receipt: %s", receipt.Ret)
	}

	ret := &pb.Interchain{}

	if err := ret.Unmarshal(receipt.Ret); err != nil {
		return nil, fmt.Errorf("unmarshal interchain meta from bitxhub: %w", err)
	}

	return ret, nil
}

func getAppchainInfo(client rpcx.Client) (*rpcx.Appchain, error) {
	receipt, err := client.InvokeBVMContract(constant.AppchainMgrContractAddr.Address(), "Appchain", nil)
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
