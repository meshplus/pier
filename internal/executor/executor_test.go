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
	"github.com/meshplus/pier/pkg/model"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from         = "did:bitxhub:fabappchain:."
	fromContract = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to           = "did:bitxhub:ethappchain:."
	toContract   = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50161"
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
	receipt, err := exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: ibtp1, IsValid: true})
	require.Nil(t, err)
	require.NotNil(t, receipt)

	receipt, err = exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: ibtp1Receipt, IsValid: true})
	require.Nil(t, err)
	require.Nil(t, receipt)

	// test for ibtp execute failure
	receipt, err = exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: ibtp2, IsValid: true})
	require.Nil(t, err)
	require.NotNil(t, receipt)

	receipt, err = exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: ibtp2Receipt, IsValid: true})
	require.Nil(t, err)
	require.Nil(t, receipt)

	// test for empty ibtp execution
	receipt, err = exec.ExecuteIBTP(nil)
	require.NotNil(t, err)

	// test for wrong type ibtp execution
	receipt, err = exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: wrongTypeIbtp, IsValid: true})
	require.NotNil(t, err)

	// test for submit error ibtp execution
	require.Panics(t, func() {
		exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: submitErrorIbtp, IsValid: true})
	})

	// test for empty response ibtp execution
	require.Panics(t, func() {
		exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: emptyRespIbtp, IsValid: true})
	})

	// test for encrypted ibtp execution
	receipt, err = exec.ExecuteIBTP(&model.WrappedIBTP{Ibtp: encryptedIbtpReceipt, IsValid: true})
	require.Nil(t, err)

	// test QueryInterchainMeta
	meta := exec.QueryInterchainMeta()
	require.Equal(t, uint64(1), meta[from])

	// test QueryCallbackMeta
	meta = exec.QueryCallbackMeta()
	require.Equal(t, uint64(2), meta[to])

	time.Sleep(500 * time.Microsecond)
	require.Nil(t, exec.Stop())
}

func TestQueryReceipt(t *testing.T) {
	exec, cli := prepare(t)
	defer exec.storage.Close()

	originalIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN, false)
	receiptIBTP := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS, false)

	cli.EXPECT().GetReceipt(originalIBTP).Return(receiptIBTP, nil).AnyTimes()

	// test for normal ibtp receipt
	receipt, err := exec.QueryIBTPReceipt(originalIBTP)
	require.Nil(t, err)

	require.Equal(t, originalIBTP.From, receipt.From)
	require.Equal(t, originalIBTP.To, receipt.To)
	require.Equal(t, originalIBTP.Index, receipt.Index)

	// test for nil ibtp receipt
	receipt, err = exec.QueryIBTPReceipt(nil)
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
		SrcContractId: fromContract,
		DstContractId: toContract,
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
		SrcContractId: fromContract,
		DstContractId: toContract,
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
