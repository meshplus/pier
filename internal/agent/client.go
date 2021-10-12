package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/pkg/plugins"
)

// BxhClient represents plugin client fo relaychain, by packing
// operations of relaychain to an appchain plugin-client, we can
// easily excute ibtp.Payload on relaychain just like what we used
// to do on an appchain.
type BxhClient struct {
	agent rpcx.Client
	appchainmgr.Appchain
}

func (client *BxhClient) GetIBTPCh() chan *pb.IBTP {
	panic("implement me")
}

func (client *BxhClient) GetUpdateMeta() chan *pb.UpdateMeta {
	panic("implement me")
}

func (client *BxhClient) SubmitIBTP(from string, index uint64, serviceID string, ibtpType pb.IBTP_Type, content *pb.Content, proof *pb.BxhProof, isEncrypted bool) (*pb.SubmitIBTPResponse, error) {
	panic("implement me")
}

func (client *BxhClient) SubmitReceipt(to string, index uint64, serviceID string, ibtpType pb.IBTP_Type, result *pb.Result, proof *pb.BxhProof) (*pb.SubmitIBTPResponse, error) {
	panic("implement me")
}

var _ plugins.Client = (*BxhClient)(nil)

// CreateClient creates plugin client from agent
func CreateClient(agent rpcx.Client) *BxhClient {
	return &BxhClient{
		agent: agent,
	}
}

// SubmitIBTP submits the inter-relaychain ibtp to relaychain to execute ibtp.Payload
//func (client *BxhClient) SubmitIBTP(ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
//	ret := &pb.SubmitIBTPResponse{}
//	ret.Status = false
//	payload := &pb.Payload{}
//	if err := payload.Unmarshal(ibtp.Payload); err != nil {
//		return ret, fmt.Errorf("ibtp payload unmarshal: %w", err)
//	}
//	content := &pb.Content{}
//	if err := content.Unmarshal(payload.Content); err != nil {
//		return ret, fmt.Errorf("ibtp content unmarshal: %w", err)
//	}
//
//	args := []*pb.Arg{}
//	//args = append(args, rpcx.String(content.DstContractId))
//	args = append(args, rpcx.String(content.Func))
//	realArg, err := json.Marshal(content.Args)
//	if err != nil {
//		return ret, fmt.Errorf("args marshal: %w", err)
//	}
//	args = append(args, rpcx.Bytes(realArg))
//
//	receipt, err := client.agent.InvokeContract(
//		pb.TransactionData_BVM,
//		constant.InterRelayBrokerContractAddr.Address(), // broker
//		"InvokeInterRelayContract",
//		nil,
//		args...)
//	if err != nil {
//		return ret, fmt.Errorf("invoke payload: %w", err)
//	}
//
//	ret.Status = receipt.IsSuccess()
//	ret.Message = string(receipt.Ret)
//	ret.Result = &pb.IBTP{}
//
//	newArgs := make([][]byte, 0) // TODO modify by receipt.Ret
//	switch content.Func {
//	case "Synchronize":
//		newArgs = append(newArgs, nil)
//	}
//
//	newIbtp, err := client.generateCallback(ibtp, newArgs, receipt.IsSuccess())
//	if err != nil {
//		return nil, err
//	}
//
//	newIbtp.Proof, err = client.getProof(newIbtp)
//	if err != nil {
//		return nil, err
//	}
//
//	ret.Result = newIbtp
//
//	return ret, nil
//}

// GetInMessage gets receipt by index and source chain_id
// TODO
func (client *BxhClient) GetInMessage(from string, idx uint64) ([][]byte, error) {
	return nil, nil
}

// GetInMeta gets an index map, which implicates the greatest index of
// ingoing interchain txs for each source chain
func (client *BxhClient) GetInMeta() (map[string]uint64, error) {
	receipt, err := client.agent.InvokeContract(
		pb.TransactionData_BVM,
		constant.InterRelayBrokerContractAddr.Address(),
		"GetInCouterMap",
		nil)
	if err != nil {
		return nil, err
	}
	InCounterMap := make(map[string]uint64)
	err = json.Unmarshal(receipt.Ret, &InCounterMap)
	if err != nil {
		return nil, err
	}
	return InCounterMap, nil
}

func (client *BxhClient) GetReceiptMessage(servicePair string, idx uint64) (*pb.IBTP, error) {
	return nil, nil
}

// GetCallbackMeta gets an index map, which implicates the greatest index of
// executed callback txs for each receiving chain
// TODO
func (client *BxhClient) GetCallbackMeta() (map[string]uint64, error) {
	return nil, nil
}

func (client *BxhClient) GetSrcRollbackMeta() (map[string]uint64, error) {
	return nil, nil
}

func (client *BxhClient) GetDstRollbackMeta() (map[string]uint64, error) {
	return nil, nil
}

// Initialize .
func (client *BxhClient) Initialize(configPath string, extra []byte) error {
	return nil
}

// Start .
func (client *BxhClient) Start() error {
	return nil
}

// Stop .
func (client *BxhClient) Stop() error {
	return nil
}

// GetIBTP .
func (client *BxhClient) GetIBTP() chan *pb.IBTP {
	return nil
}

// GetOutMeta .
func (client *BxhClient) GetOutMeta() (map[string]uint64, error) {
	return nil, nil
}

// GetOutMessage .
func (client *BxhClient) GetOutMessage(to string, idx uint64) (*pb.IBTP, error) {
	return nil, nil
}

// CommitCallback .
func (client *BxhClient) CommitCallback(ibtp *pb.IBTP) error {
	return nil
}

func (client *BxhClient) GetReceipt(ibtp *pb.IBTP) (*pb.IBTP, error) {
	return nil, nil
}

func (client *BxhClient) IncreaseInMeta(ibtp *pb.IBTP) (*pb.IBTP, error) {
	return nil, nil
}

func (client *BxhClient) GetServices() ([]string, error) {
	return nil, nil
}

func (client *BxhClient) GetChainID() (string, string, error) {
	return "", "", nil
}

// Name .
func (client *BxhClient) Name() string {
	return ""
}

// Type .
func (client *BxhClient) Type() string {
	return ""
}

func (client *BxhClient) getProof(ibtp *pb.IBTP) ([]byte, error) {
	var signs []byte
	if err := retry.Retry(func(attempt uint) error {
		var err error
		signs, err = client.getIBTPSigns(ibtp)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		return nil, err
		// log.NewWithModule("agent client plugin").Panic(err)
	}
	return signs, nil
}

func (client *BxhClient) getIBTPSigns(ibtp *pb.IBTP) ([]byte, error) {
	hash := ibtp.Hash()
	resp, err := client.agent.GetMultiSigns(hash.String(), pb.GetMultiSignsRequest_IBTP_REQUEST)
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for ibtp %s", ibtp.ID())
	}
	signs, err := resp.Marshal()
	if err != nil {
		return nil, err
	}

	return signs, nil
}

func (client *BxhClient) generateCallback(original *pb.IBTP, args [][]byte, status bool) (*pb.IBTP, error) {
	if original == nil {
		return nil, fmt.Errorf("got nil ibtp to generate receipt")
	}
	pd := &pb.Payload{}
	if err := pd.Unmarshal(original.Payload); err != nil {
		return nil, fmt.Errorf("ibtp payload unmarshal: %w", err)
	}

	originalContent := &pb.Content{}
	if err := originalContent.Unmarshal(pd.Content); err != nil {
		return nil, fmt.Errorf("ibtp payload unmarshal: %w", err)
	}

	content := &pb.Result{
		//Func: originalContent.Callback,
		Data: args,
	}
	b, err := content.Marshal()
	if err != nil {
		return nil, err
	}
	retPd := &pb.Payload{
		Content: b,
	}

	pdb, err := retPd.Marshal()
	if err != nil {
		return nil, err
	}

	typ := pb.IBTP_RECEIPT_SUCCESS
	if original.Type == pb.IBTP_INTERCHAIN {
		if !status {
			typ = pb.IBTP_RECEIPT_FAILURE
		}
	}

	return &pb.IBTP{
		From:    original.From,
		To:      original.To,
		Index:   original.Index,
		Type:    typ,
		Payload: pdb,
		Version: original.Version,
	}, nil
}
