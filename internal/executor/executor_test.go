package executor

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/txcrypto/mock_txcrypto"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to   = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50161"
)

func TestChannelExecutor_QueryInterchainMetaError(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)

	cli.EXPECT().GetInMeta().Return(nil, fmt.Errorf("get in meta error")).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(nil, fmt.Errorf("callback meta error")).AnyTimes()

	exec, err := New(cli, "", nil, nil, log.NewWithModule("executor"))
	require.Nil(t, err)

	// test QueryInterchainMeta
	meta := exec.QueryInterchainMeta()
	require.Equal(t, 0, len(meta))

	// test QueryCallbackMeta
	meta = exec.QueryCallbackMeta()
	require.Equal(t, 0, len(meta))
}

func TestExecute(t *testing.T) {
	exec, cli := prepare(t)
	defer exec.storage.Close()

	// set expect values
	ibtp1 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, false)
	ibtp1Receipt := getIBTPReceipt(t, uint64(1), pb.IBTP_RECEIPT_SUCCESS, false)
	ibtp2 := getIBTP(t, uint64(2), pb.IBTP_INTERCHAIN, false)
	ibtp2Receipt := getIBTPReceipt(t, uint64(2), pb.IBTP_RECEIPT_FAILURE, false)
	wrongTypeIbtp := getIBTP(t, uint64(3), -1, false)
	submitErrorIbtp := getIBTP(t, uint64(4), pb.IBTP_INTERCHAIN, false)
	emptyRespIbtp := getIBTP(t, uint64(5), pb.IBTP_INTERCHAIN, false)
	encryptedIbtpReceipt := getIBTPReceipt(t, uint64(6), pb.IBTP_RECEIPT_SUCCESS, true)
	ret1 := &pb.SubmitIBTPResponse{
		Status: true,
		Result: ibtp1Receipt,
	}
	ret2 := &pb.SubmitIBTPResponse{
		Status: false,
		Result: ibtp2Receipt,
	}
	cli.EXPECT().SubmitIBTP(ibtp1).Return(ret1, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp2).Return(ret2, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(submitErrorIbtp).Return(nil, fmt.Errorf("submit error")).AnyTimes()
	cli.EXPECT().SubmitIBTP(emptyRespIbtp).Return(nil, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp1Receipt).Return(ret1, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp2Receipt).Return(ret2, nil).AnyTimes()
	cli.EXPECT().Stop().Return(nil).AnyTimes()

	// start executor
	require.Nil(t, exec.Start())

	// test for normal ibtp execution
	receipt, err := exec.ExecuteIBTP(ibtp1)
	require.Nil(t, err)
	require.NotNil(t, receipt)

	receipt, err = exec.ExecuteIBTP(ibtp1Receipt)
	require.Nil(t, err)
	require.Nil(t, receipt)

	// test for ibtp execute failure
	receipt, err = exec.ExecuteIBTP(ibtp2)
	require.Nil(t, err)
	require.NotNil(t, receipt)

	receipt, err = exec.ExecuteIBTP(ibtp2Receipt)
	require.Nil(t, err)
	require.Nil(t, receipt)

	// test for empty ibtp execution
	receipt, err = exec.ExecuteIBTP(nil)
	require.NotNil(t, err)

	// test for wrong type ibtp execution
	receipt, err = exec.ExecuteIBTP(wrongTypeIbtp)
	require.NotNil(t, err)

	// test for submit error ibtp execution
	require.Panics(t, func() {
		exec.ExecuteIBTP(submitErrorIbtp)
	})

	// test for empty response ibtp execution
	require.Panics(t, func() {
		exec.ExecuteIBTP(emptyRespIbtp)
	})

	// test for encrypted ibtp execution
	receipt, err = exec.ExecuteIBTP(encryptedIbtpReceipt)
	require.Nil(t, err)

	// test QueryInterchainMeta
	meta := exec.QueryInterchainMeta()
	require.Equal(t, uint64(1), meta[types.NewAddressByStr(from).String()])

	// test QueryCallbackMeta
	meta = exec.QueryCallbackMeta()
	require.Equal(t, uint64(2), meta[types.NewAddressByStr(to).String()])

	time.Sleep(500 * time.Microsecond)
	require.Nil(t, exec.Stop())
}

func TestQueryReceipt(t *testing.T) {
	exec, cli := prepare(t)
	defer exec.storage.Close()

	originalIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN, false)
	cryptoIBTP := getIBTP(t, 3, pb.IBTP_INTERCHAIN, true)
	cryptoAssetIBTP := getIBTP(t, 3, pb.IBTP_ASSET_EXCHANGE_INIT, true)
	decryptErrorIBTP := getIBTP(t, 5, pb.IBTP_INTERCHAIN, true)
	decryptErrorIBTP.From = "decryptError"
	encryptErrorIBTP := getIBTP(t, 5, pb.IBTP_INTERCHAIN, true)
	encryptErrorIBTP.To = "encryptError"
	args := [][]byte{[]byte("Alice"), []byte("100")}
	args3 := [][]byte{[]byte("true")}
	args4 := [][]byte{[]byte("false")}

	cli.EXPECT().GetInMessage(from, uint64(1)).Return(args, nil).AnyTimes()
	cli.EXPECT().GetInMessage(from, uint64(2)).Return(nil, fmt.Errorf("get in message error")).AnyTimes()
	cli.EXPECT().GetInMessage(from, uint64(3)).Return(args3, nil).AnyTimes()
	cli.EXPECT().GetInMessage(from, uint64(4)).Return(args4, nil).AnyTimes()

	// test for normal ibtp receipt
	receipt, err := exec.QueryIBTPReceipt(from, 1, originalIBTP)
	require.Nil(t, err)

	require.Equal(t, originalIBTP.From, receipt.From)
	require.Equal(t, originalIBTP.To, receipt.To)
	require.Equal(t, originalIBTP.Index, receipt.Index)

	receiptPd := &pb.Payload{}
	require.Nil(t, receiptPd.Unmarshal(receipt.Payload))

	receiptContent := &pb.Content{}
	require.Nil(t, receiptContent.Unmarshal(receiptPd.Content))

	originalPd := &pb.Payload{}
	require.Nil(t, originalPd.Unmarshal(originalIBTP.Payload))

	originalContent := &pb.Content{}
	require.Nil(t, originalContent.Unmarshal(originalPd.Content))

	require.Equal(t, receiptContent.Func, originalContent.Callback)
	require.Equal(t, receiptContent.Args[1:], args)

	// test for nil ibtp receipt
	receipt, err = exec.QueryIBTPReceipt(from, 1, nil)
	require.NotNil(t, err)

	// test for get in message error ibtp receipt
	receipt, err = exec.QueryIBTPReceipt(from, 2, originalIBTP)
	require.NotNil(t, err)

	// test for encrypted right but args parsing bool error
	receipt, err = exec.QueryIBTPReceipt(from, 1, cryptoIBTP)
	require.NotNil(t, err)

	// test for encrypted right and args true
	receipt, err = exec.QueryIBTPReceipt(from, 3, cryptoIBTP)
	require.Nil(t, err)
	require.Equal(t, pb.IBTP_RECEIPT_SUCCESS, receipt.Type)

	// test for encrypted right and args false
	receipt, err = exec.QueryIBTPReceipt(from, 4, cryptoIBTP)
	require.Nil(t, err)
	require.Equal(t, pb.IBTP_RECEIPT_FAILURE, receipt.Type)

	// test for encrypted right payload asset IBTP
	receipt, err = exec.QueryIBTPReceipt(from, 1, cryptoAssetIBTP)
	require.Nil(t, err)
	require.Equal(t, pb.IBTP_ASSET_EXCHANGE_RECEIPT, receipt.Type)

	// test for decrypt error IBTP
	receipt, err = exec.QueryIBTPReceipt(from, 1, decryptErrorIBTP)
	require.NotNil(t, err)

	// test for encrypt error IBTP
	receipt, err = exec.QueryIBTPReceipt(from, 1, encryptErrorIBTP)
	require.NotNil(t, err)
}

func prepare(t *testing.T) (*ChannelExecutor, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	cryptor := mock_txcrypto.NewMockCryptor(mockCtl)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)
	meta := &rpcx.Interchain{
		ID: from,
	}

	ct := &pb.Content{
		SrcContractId: from,
		DstContractId: to,
		Func:          "interchainCharge",
		Args:          [][]byte{[]byte("Alice")},
		Callback:      "interchainConfirm",
	}
	c, err := ct.Marshal()

	inMeta := map[string]uint64{from: uint64(1)}
	callbackMeta := map[string]uint64{to: uint64(2)}

	cli.EXPECT().GetInMeta().Return(inMeta, nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(callbackMeta, nil).AnyTimes()
	cryptor.EXPECT().Decrypt(gomock.Any(), from).Return(c, nil).AnyTimes()
	cryptor.EXPECT().Decrypt(gomock.Any(), "decryptError").Return(nil, fmt.Errorf("decrypt error")).AnyTimes()
	cryptor.EXPECT().Encrypt(gomock.Any(), to).Return(c, nil).AnyTimes()
	cryptor.EXPECT().Encrypt(gomock.Any(), "encryptError").Return(nil, fmt.Errorf("encrypt error")).AnyTimes()
	exec, err := New(cli, meta.ID, storage, cryptor, log.NewWithModule("executor"))
	require.Nil(t, err)
	return exec, cli
}

func getReceipt() *pb.Receipt {
	return &pb.Receipt{
		Version: []byte("0.4.5"),
		TxHash:  &types.Hash{},
		Ret:     nil,
		Status:  0,
	}
}

func getIBTPReceipt(t *testing.T, index uint64, typ pb.IBTP_Type, encrypted bool) *pb.IBTP {
	receipt := getIBTP(t, index, typ, encrypted)
	receipt.From, receipt.To = receipt.To, receipt.From
	return receipt
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type, encrypted bool) *pb.IBTP {
	ct := &pb.Content{
		SrcContractId: from,
		DstContractId: to,
		Func:          "set",
		Args:          [][]byte{[]byte("Alice")},
		Callback:      "interchainConfirm",
	}
	c, err := ct.Marshal()
	require.Nil(t, err)

	pd := pb.Payload{
		Encrypted: encrypted,
		Content:   c,
	}
	ibtppd, err := pd.Marshal()
	require.Nil(t, err)

	return &pb.IBTP{
		From:      from,
		To:        to,
		Payload:   ibtppd,
		Index:     index,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
	}
}
