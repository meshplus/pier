package utils

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	network_pb "github.com/meshplus/go-lightp2p/pb"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
)

type TransactOpts struct {
	From  string
	Nonce uint64
}

var (
	// error type which can be fixed by retrying
	ErrRecoverable = errors.New("recoverable error")

	// error type which tx format is invalid to send
	ErrReconstruct = errors.New("invalid tx format error")

	// set ibtp and normal nonce at the same time
	ErrIllegalNonceSet = fmt.Errorf("%w: can't set ibtp nonce and normal nonce at the same time", ErrReconstruct)

	// signature for tx is invalid
	ErrSignTx = fmt.Errorf("%w: sign for transaction invalid", ErrReconstruct)

	// network problem received from grpc
	ErrBrokenNetwork = fmt.Errorf("%w: grpc broker error", ErrRecoverable)
)

func GenerateBatchHeaderRequest(begin, end uint64) []*pb.GetBlockHeaderRequest {
	requests := make([]*pb.GetBlockHeaderRequest, 0)
	var batchEnd uint64
	batchStart := begin
	for end-batchStart+1 >= 10 {
		batchEnd = batchStart + 9
		req := &pb.GetBlockHeaderRequest{Begin: batchStart, End: batchEnd}
		requests = append(requests, req)
		batchStart = batchEnd + 1
	}
	if end-batchStart >= 0 {
		req := &pb.GetBlockHeaderRequest{Begin: batchStart, End: end}
		requests = append(requests, req)
	}
	return requests
}

func GenerateBatchWrappersRequest(begin, end uint64, did string) []*pb.GetInterchainTxWrappersRequest {
	requests := make([]*pb.GetInterchainTxWrappersRequest, 0)
	var batchEnd uint64
	batchStart := begin
	for end-batchStart+1 >= 10 {
		batchEnd = batchStart + 9
		req := &pb.GetInterchainTxWrappersRequest{
			Begin: batchStart,
			End:   batchEnd,
			Pid:   did}
		requests = append(requests, req)
		batchStart = batchEnd + 1
	}
	if end-batchStart >= 0 {
		req := &pb.GetInterchainTxWrappersRequest{
			Begin: batchStart,
			End:   end,
			Pid:   did}
		requests = append(requests, req)
	}
	return requests
}
func SenView(peerMgr peermgr.PeerManager, tx *pb.BxhTransaction) (*pb.Receipt, error) {
	req, err := tx.Marshal()
	if err != nil {
		return nil, fmt.Errorf("tx marshal error: %v", err)
	}
	msg := peermgr.Message(pb.Message_PIER_SEND_VIEW, true, req)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("message marshal err: %w", err)
	}
	resp, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	receipt := &pb.Receipt{}
	err = receipt.Unmarshal(resp)
	if err != nil {
		return nil, fmt.Errorf("receipt unmarshal :%v\n err msg is: %s", err, string(resp))
	}
	return receipt, nil
}

func GenerateIBTPTx(priv crypto.PrivateKey, ibtp *pb.IBTP) (*pb.BxhTransaction, error) {
	if ibtp == nil {
		return nil, fmt.Errorf("empty ibtp not allowed")
	}
	from, err := priv.PublicKey().Address()
	if err != nil {
		return nil, err
	}

	tx := &pb.BxhTransaction{
		From:      from,
		To:        constant.InterchainContractAddr.Address(),
		IBTP:      ibtp,
		Timestamp: time.Now().UnixNano(),
	}

	return tx, nil
}

func GenerateContractTx(priv crypto.PrivateKey, vmType pb.TransactionData_VMType, address *types.Address, method string, args ...*pb.Arg) (*pb.BxhTransaction, error) {
	from, err := priv.PublicKey().Address()
	if err != nil {
		return nil, err
	}

	pl := &pb.InvokePayload{
		Method: method,
		Args:   args[:],
	}

	data, err := pl.Marshal()
	if err != nil {
		return nil, err
	}

	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		VmType:  vmType,
		Payload: data,
	}

	payload, err := td.Marshal()
	if err != nil {
		return nil, err
	}

	tx := &pb.BxhTransaction{
		From:      from,
		To:        address,
		Payload:   payload,
		Timestamp: time.Now().UnixNano(),
	}

	return tx, nil
}

// InvokeContract let client invoke the wasm contract with specific method.
func InvokeContract(privateKey crypto.PrivateKey, peerMgr peermgr.PeerManager, vmType pb.TransactionData_VMType, address *types.Address, method string,
	opts *TransactOpts, args ...*pb.Arg) (*pb.Receipt, error) {
	from, err := privateKey.PublicKey().Address()
	if err != nil {
		return nil, err
	}

	pl := &pb.InvokePayload{
		Method: method,
		Args:   args[:],
	}

	data, err := pl.Marshal()
	if err != nil {
		return nil, err
	}

	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		VmType:  vmType,
		Payload: data,
	}

	payload, err := td.Marshal()
	if err != nil {
		return nil, err
	}

	tx := &pb.BxhTransaction{
		From:      from,
		To:        address,
		Payload:   payload,
		Timestamp: time.Now().UnixNano(),
	}

	return sendTransactionWithReceipt(privateKey, peerMgr, tx, opts)
}
func sendTransactionWithReceipt(privateKey crypto.PrivateKey, peerMgr peermgr.PeerManager, tx *pb.BxhTransaction, opts *TransactOpts) (*pb.Receipt, error) {
	hash, err := sendTransaction(privateKey, peerMgr, tx, opts)
	if err != nil {
		return nil, fmt.Errorf("send tx error: %w", err)
	}

	receipt, err := GetReceipt(peerMgr, hash)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func SendTransaction(privateKey crypto.PrivateKey, peerMgr peermgr.PeerManager, tx *pb.BxhTransaction, opts *TransactOpts) (string, error) {
	return sendTransaction(privateKey, peerMgr, tx, opts)
}

func sendTransaction(privateKey crypto.PrivateKey, peerMgr peermgr.PeerManager, tx *pb.BxhTransaction, opts *TransactOpts) (string, error) {
	if tx.From == nil {
		return "", fmt.Errorf("%w: from address can't be empty", ErrReconstruct)
	}
	if opts == nil {
		opts = new(TransactOpts)
		opts.From = tx.From.String() // set default from for opts
	}
	var (
		nonce uint64
		err   error
	)
	if opts.Nonce == 0 {
		// no nonce set for tx, then use latest nonce from bitxhub
		nonce, err = GetPendingNonceByAccount(peerMgr, opts.From)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve nonce for account %s for %s", opts.From, err.Error())
		}
	} else {
		nonce = opts.Nonce
	}
	tx.Nonce = nonce

	if err := tx.Sign(privateKey); err != nil {
		return "", fmt.Errorf("%w: for reason %s", ErrSignTx, err.Error())
	}

	req, err := tx.Marshal()
	if err != nil {
		return "", err
	}
	msg := peermgr.Message(pb.Message_PIER_SEND_TRANSACTION, true, req)
	data, err := msg.Marshal()
	if err != nil {
		return "", fmt.Errorf("sendTransaction marshal err: %w", err)
	}
	resp, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return "", fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	// todo(lrx): gogofaster generate umarshal function can not catch different types of unmarshal errors
	msgTmp := &network_pb.Message{}
	err = msgTmp.Unmarshal(resp)
	if err == nil && msgTmp.Msg == network_pb.Message_ERROR {
		return "", fmt.Errorf("sendTransaction err:%s", string(msgTmp.Data))
	}
	response := &pb.TransactionHashMsg{}
	// if resp type is Message, unmarshal err is nil, it is incomprehensible
	err = response.Unmarshal(resp)
	if err != nil {
		return "", fmt.Errorf("umarshal sendTransaction err:%w\n err msg is: %s", err, string(resp))
	}
	return response.TxHash, err
}

func GetPendingNonceByAccount(peerMgr peermgr.PeerManager, account string) (uint64, error) {
	req := &pb.Address{Address: account}
	rb, err := req.Marshal()
	if err != nil {
		return 0, err
	}
	msg := peermgr.Message(pb.Message_PIER_GET_PENDING_NONCE_BY_ACCOUNT, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return 0, fmt.Errorf("GetPendingNonceByAccount marshal err: %w", err)
	}
	res, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return 0, fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	msgTmp := &network_pb.Message{}
	err = msgTmp.Unmarshal(res)
	if err == nil && msgTmp.Msg == network_pb.Message_ERROR {
		return 0, fmt.Errorf("GetPendingNonceByAccount err:%s", string(msgTmp.Data))
	}
	resp := &pb.Response{}
	err = resp.Unmarshal(res)
	if err != nil {
		return 0, fmt.Errorf("umarshal GetPendingNonceByAccount err:%w\n err msg is: %s", err, string(res))
	}
	return strconv.ParseUint(string(resp.Data), 10, 64)
}

// GetReceipt get receipt by tx hash
func GetReceipt(peerMgr peermgr.PeerManager, hash string) (*pb.Receipt, error) {
	var receipt *pb.Receipt
	var err error
	err = retry.Retry(func(attempt uint) error {
		receipt, err = getReceipt(peerMgr, hash)
		if err != nil {
			return err
		}

		return nil
	},
		strategy.Limit(5),
		strategy.Backoff(backoff.Fibonacci(2*time.Second)),
	)

	if err != nil {
		return nil, err
	}

	return receipt, nil
}

func getReceipt(peerMgr peermgr.PeerManager, hash string) (*pb.Receipt, error) {
	req := &pb.TransactionHashMsg{
		TxHash: hash,
	}
	rb, err := req.Marshal()
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_PIER_GET_RECEIPT, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("getReceipt marshal err: %w", err)
	}
	res, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	resp := &pb.Receipt{}
	err = resp.Unmarshal(res)
	if err != nil {
		return nil, fmt.Errorf("umarshal getReceipt err:%w\n err msg is: %s", err, string(res))
	}
	return resp, nil
}

func GetTransaction(peerMgr peermgr.PeerManager, hash string) (*pb.GetTransactionResponse, error) {
	req := &pb.TransactionHashMsg{
		TxHash: hash,
	}
	rb, err := req.Marshal()
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_PIER_GET_TRANSACTION, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("GetTransaction marshal err: %w", err)
	}
	res, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	response := &pb.GetTransactionResponse{}
	err = response.Unmarshal(res)
	if err != nil {
		return nil, fmt.Errorf("umarshal GetTransaction err:%w\n err msg is: %s", err, string(res))
	}
	return response, nil
}

func GetMultiSigns(peerMgr peermgr.PeerManager, content string, typ pb.GetMultiSignsRequest_Type) (*pb.SignResponse, error) {
	req := &pb.GetMultiSignsRequest{
		Content: content,
		Type:    typ,
	}
	rb, err := req.Marshal()
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_PIER_GET_MULTI_SIGNS, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("message marshal err: %w", err)
	}
	resp, err := peerMgr.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("%s, %w", err.Error(), ErrBrokenNetwork)
	}
	response := &pb.SignResponse{}
	err = response.Unmarshal(resp)
	if err != nil {
		return nil, fmt.Errorf("umarshal GetMultiSigns err:%w\n err msg is: %s", err, string(resp))
	}
	return response, nil
}
