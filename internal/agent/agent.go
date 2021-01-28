package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
)

// agent is responsible for interacting with bitxhub
var (
	_               Agent = (*BxhAgent)(nil)
	ErrIBTPNotFound       = fmt.Errorf("receipt from bitxhub failed")
)

// BxhAgent represents the necessary data for interacting with bitxhub
type BxhAgent struct {
	client     rpcx.Client
	from       types.Address
	addr       string
	validators string
}

// New create an instance of BxhAgent given the client of bitxhub and
// the appchain id and some configuration of bitxhub
func New(client rpcx.Client, pierID types.Address, bitxhub repo.Relay) (*BxhAgent, error) {
	return &BxhAgent{
		client:     client,
		from:       pierID,
		addr:       bitxhub.Addr,
		validators: strings.Join(bitxhub.Validators, ","),
	}, nil
}

func (agent *BxhAgent) Stop() error {
	return agent.client.Stop()
}

// Appchain implements Agent
func (agent *BxhAgent) Appchain() (*rpcx.Appchain, error) {
	receipt, err := agent.client.InvokeBVMContract(constant.AppchainMgrContractAddr.Address(), "Appchain", nil)
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

func (agent *BxhAgent) GetInterchainMeta() (*pb.Interchain, error) {
	receipt, err := agent.client.InvokeBVMContract(constant.InterchainContractAddr.Address(), "Interchain", nil)
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

func (agent *BxhAgent) SyncBlockHeader(ctx context.Context, headerCh chan *pb.BlockHeader) error {
	ch, err := agent.client.Subscribe(ctx, pb.SubscriptionRequest_BLOCK_HEADER, nil)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-ch:
				if !ok {
					close(headerCh)
					return
				}
				headerCh <- h.(*pb.BlockHeader)
			}
		}
	}()

	return nil
}

func (agent *BxhAgent) GetBlockHeader(ctx context.Context, begin, end uint64, ch chan *pb.BlockHeader) error {
	if err := agent.client.GetBlockHeader(ctx, begin, end, ch); err != nil {
		return err
	}

	return nil
}

func (agent *BxhAgent) SyncInterchainTxWrappers(ctx context.Context, txCh chan *pb.InterchainTxWrappers) error {
	subscriptType := pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER
	ch, err := agent.client.Subscribe(ctx, subscriptType, []byte(agent.from.String()))
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-ch:
				if !ok {
					close(txCh)
					return
				}
				txCh <- h.(*pb.InterchainTxWrappers)
			}
		}
	}()

	return nil
}

func (agent *BxhAgent) SyncUnionInterchainTxWrappers(ctx context.Context, txCh chan *pb.InterchainTxWrappers) error {
	subscriptType := pb.SubscriptionRequest_UNION_INTERCHAIN_TX_WRAPPER
	ch, err := agent.client.Subscribe(ctx, subscriptType, []byte(agent.from.String()))
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-ch:
				if !ok {
					close(txCh)
					return
				}
				txCh <- h.(*pb.InterchainTxWrappers)
			}
		}
	}()

	return nil
}

// GetInterchainTxWrapper implements Agent
func (agent *BxhAgent) GetInterchainTxWrappers(ctx context.Context, begin, end uint64, ch chan *pb.InterchainTxWrappers) error {
	return agent.client.GetInterchainTxWrappers(ctx, agent.from.String(), begin, end, ch)
}

// SendTransaction implements Agent
func (agent *BxhAgent) SendTransaction(tx *pb.Transaction) (*pb.Receipt, error) {
	return agent.client.SendTransactionWithReceipt(tx, nil)
}

// InvokeContract implements Agent
func (agent *BxhAgent) InvokeContract(vmType pb.TransactionData_VMType, address *types.Address, method string, opts *rpcx.TransactOpts, args ...*pb.Arg) (*pb.Receipt, error) {
	return agent.client.InvokeContract(vmType, address, method, opts, args...)
}

// SendIBTP implements Agent
func (agent *BxhAgent) SendIBTP(ibtp *pb.IBTP) (*pb.Receipt, error) {
	proof := ibtp.GetProof()
	proofHash := sha256.Sum256(proof)
	ibtp.Proof = proofHash[:]

	tx, err := agent.client.GenerateIBTPTx(ibtp)
	if err != nil {
		return nil, fmt.Errorf("generate ibtp tx error:%v", err)
	}
	tx.Extra = proof
	return agent.client.SendTransactionWithReceipt(tx, &rpcx.TransactOpts{
		From:      fmt.Sprintf("%s-%s-%d", ibtp.From, ibtp.To, ibtp.Category()),
		IBTPNonce: ibtp.Index,
	})
}

// GetReceipt implements Agent
func (agent *BxhAgent) GetReceipt(hash string) (*pb.Receipt, error) {
	return agent.client.GetReceipt(hash)
}

// GetIBTPByID implements Agent
func (agent *BxhAgent) GetIBTPByID(id string) (*pb.IBTP, error) {
	receipt, err := agent.client.InvokeContract(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetIBTPByID", nil, rpcx.String(id))
	if err != nil {
		return nil, err
	}

	if !receipt.IsSuccess() {
		return nil, fmt.Errorf("%w: %s", ErrIBTPNotFound, string(receipt.Ret))
	}
	hash := types.NewHash(receipt.Ret)

	response, err := agent.client.GetTransaction(hash.String())
	if err != nil {
		return nil, err
	}
	ibtp := response.Tx.GetIBTP()
	if ibtp == nil {
		return nil, fmt.Errorf("empty ibtp from bitxhub")
	}
	return ibtp, nil
}

// GetChainMeta implements Agent
func (agent *BxhAgent) GetChainMeta() (*pb.ChainMeta, error) {
	return agent.client.GetChainMeta()
}

func (agent *BxhAgent) GetAssetExchangeSigns(id string) ([]byte, error) {
	resp, err := agent.client.GetMultiSigns(id, pb.GetMultiSignsRequest_ASSET_EXCHANGE)
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for asset exchange id %s", id)
	}

	var signs []byte
	for _, sign := range resp.Sign {
		signs = append(signs, sign...)
	}

	return signs, nil
}

func (agent *BxhAgent) GetIBTPSigns(ibtp *pb.IBTP) ([]byte, error) {
	hash := ibtp.Hash()
	resp, err := agent.client.GetMultiSigns(hash.String(), pb.GetMultiSignsRequest_IBTP)
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

func (agent *BxhAgent) GetAppchains() ([]*rpcx.Appchain, error) {
	tx, err := agent.client.GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), "Appchains")
	if err != nil {
		return nil, err
	}
	tx.Nonce = 1 // just to pass check
	receipt, err := agent.client.SendView(tx)
	if err != nil {
		return nil, err
	}
	ret := make([]*rpcx.Appchain, 0)
	if receipt.Ret == nil {
		return ret, nil
	}
	if err := json.Unmarshal(receipt.Ret, &ret); err != nil {
		return nil, err
	}
	appchains := make([]*rpcx.Appchain, 0)
	for _, appchain := range ret {
		if appchain.ChainType != repo.BitxhubType {
			appchains = append(appchains, appchain)
		}
	}
	return appchains, nil
}

func (agent *BxhAgent) GetInterchainById(from string) *pb.Interchain {
	ic := &pb.Interchain{}
	tx, err := agent.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(), "GetInterchain", rpcx.String(from))
	if err != nil {
		return ic
	}
	tx.Nonce = 1 // just to pass check
	receipt, err := agent.client.SendView(tx)
	if err != nil {
		return ic
	}
	var interchain pb.Interchain
	if err := interchain.Unmarshal(receipt.Ret); err != nil {
		return ic
	}
	return &interchain
}

func (agent *BxhAgent) GetPendingNonceByAccount(account string) (uint64, error) {
	return agent.client.GetPendingNonceByAccount(account)
}
