package executor

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/internal/txcrypto/mock_txcrypto"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to   = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50161"
)

func TestNew_GetInMetaError(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)

	cli.EXPECT().GetInMeta().Return(nil, fmt.Errorf("get in meta error")).AnyTimes()
	_, err := New(cli, "", nil, nil)
	require.NotNil(t, err)
}

func TestNew_GetCallbackMetaError(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)

	cli.EXPECT().GetInMeta().Return(make(map[string]uint64), nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(nil, fmt.Errorf("get callback meta error")).AnyTimes()
	_, err := New(cli, "", nil, nil)
	require.NotNil(t, err)
}

func TestChannelExecutor_Start(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cryptor := mock_txcrypto.NewMockCryptor(mockCtl)
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)
	meta := &rpcx.Interchain{
		ID: from,
	}
	inMeta := map[string]uint64{from: 1}
	callbackMeta := map[string]uint64{from: 1}

	cli := mock_client.NewMockClient(mockCtl)
	cli.EXPECT().GetInMeta().Return(inMeta, nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(callbackMeta, nil).AnyTimes()
	exec, err := New(cli, meta.ID, storage, cryptor)
	require.Nil(t, err)

	// start executor
	require.Nil(t, exec.Start())
}

func TestExecute(t *testing.T) {
	exec, ag, cli := prepare(t)
	defer exec.storage.Close()

	// set expect values
	ibtp1 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, false)
	ibtp1Receipt := getIBTPReceipt(t, uint64(1), pb.IBTP_RECEIPT_SUCCESS)
	ibtp2 := getIBTP(t, uint64(2), pb.IBTP_INTERCHAIN, false)
	ibtp2Receipt := getIBTPReceipt(t, uint64(2), pb.IBTP_RECEIPT_FAILURE)
	ibtp3 := getIBTP(t, uint64(3), pb.IBTP_ASSET_EXCHANGE_INIT, false)
	ibtp3Receipt := getIBTPReceipt(t, uint64(3), pb.IBTP_ASSET_EXCHANGE_RECEIPT)
	ibtp4 := getIBTP(t, uint64(4), pb.IBTP_ASSET_EXCHANGE_REDEEM, false)
	ibtp4Receipt := getIBTPReceipt(t, uint64(4), pb.IBTP_ASSET_EXCHANGE_RECEIPT)
	ibtp5 := getIBTP(t, uint64(5), pb.IBTP_ASSET_EXCHANGE_REFUND, false)
	ibtp5Receipt := getIBTPReceipt(t, uint64(5), pb.IBTP_ASSET_EXCHANGE_RECEIPT)
	wrongTypeIbtp := getIBTP(t, uint64(6), -1, false)
	wrongIndexedIbtp := getIBTP(t, uint64(4), pb.IBTP_INTERCHAIN, false)
	wrongIndexedIbtpReceipt := getIBTPReceipt(t, uint64(2), pb.IBTP_RECEIPT_SUCCESS)
	ret1 := &pb.SubmitIBTPResponse{
		Status: true,
		Result: ibtp1Receipt,
	}
	ret2 := &pb.SubmitIBTPResponse{
		Status: false,
		Result: ibtp2Receipt,
	}
	ret3 := &pb.SubmitIBTPResponse{
		Status: true,
		Result: ibtp3Receipt,
	}
	ret4 := &pb.SubmitIBTPResponse{
		Status: true,
		Result: ibtp4Receipt,
	}
	ret5 := &pb.SubmitIBTPResponse{
		Status: true,
		Result: ibtp5Receipt,
	}
	ag.EXPECT().SendIBTP(gomock.Any()).Return(getReceipt(), nil).AnyTimes()
	ag.EXPECT().GetIBTPByID(gomock.Any()).Return(getIBTP(t, 2, pb.IBTP_INTERCHAIN, false), nil).Times(1)
	cli.EXPECT().SubmitIBTP(ibtp1).Return(ret1, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp2).Return(ret2, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp3).Return(ret3, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp4).Return(ret4, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp5).Return(ret5, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp1Receipt).Return(ret1, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp2Receipt).Return(ret2, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp3Receipt).Return(ret3, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp4Receipt).Return(ret4, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(ibtp5Receipt).Return(ret5, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(wrongIndexedIbtp).Return(ret1, nil).AnyTimes()
	cli.EXPECT().SubmitIBTP(wrongIndexedIbtpReceipt).Return(ret1, nil).AnyTimes()
	cli.EXPECT().Stop().Return(nil).AnyTimes()

	// start executor
	require.Nil(t, exec.Start())

	// test for empty ibtp
	require.Nil(t, exec.HandleIBTP(nil))

	// test for normal ibtp execution
	require.NotNil(t, exec.HandleIBTP(ibtp1))
	require.Nil(t, exec.HandleIBTP(ibtp1Receipt))
	meta := exec.QueryLatestMeta()

	v, _ := meta.Load(from)
	require.Equal(t, uint64(1), v.(uint64))

	// test for replayed ibtp and receipt
	replayedIBTP := ibtp1
	replayedIBTPReceipt := ibtp1Receipt
	require.Nil(t, exec.HandleIBTP(replayedIBTP))
	require.Nil(t, exec.HandleIBTP(replayedIBTPReceipt))
	meta = exec.QueryLatestMeta()
	v, _ = meta.Load(from)
	require.Equal(t, uint64(1), v.(uint64))
	//callbackMeta :
	v, _ = exec.callbackMeta.Load(from)
	require.Equal(t, uint64(1), v.(uint64))

	// test for ibtp execute failure
	require.NotNil(t, exec.HandleIBTP(ibtp2))
	require.Nil(t, exec.HandleIBTP(ibtp2Receipt))
	meta = exec.QueryLatestMeta()

	v, _ = meta.Load(from)
	require.Equal(t, uint64(2), v.(uint64))

	v, _ = exec.callbackMeta.Load(from)
	require.Equal(t, uint64(2), v.(uint64))

	// test for wrong index ibtp and receipt
	//require.Nil(t, exec.HandleIBTP(wrongIndexedIbtp))
	require.Panics(t, func() {
		exec.HandleIBTP(wrongIndexedIbtp)
	})
	require.Nil(t, exec.HandleIBTP(wrongIndexedIbtpReceipt))

	meta = exec.QueryLatestMeta()
	v, _ = meta.Load(from)
	require.Equal(t, uint64(2), v.(uint64))

	callbackMeta := exec.QueryLatestCallbackMeta()
	v, _ = callbackMeta.Load(from)
	require.Equal(t, uint64(2), v.(uint64))

	// test for asset init ibtp execution
	require.NotNil(t, exec.HandleIBTP(ibtp3))
	require.Nil(t, exec.HandleIBTP(ibtp3Receipt))
	meta = exec.QueryLatestMeta()

	v, _ = meta.Load(from)
	require.Equal(t, uint64(3), v.(uint64))

	// test for asset redeem ibtp execution
	require.NotNil(t, exec.HandleIBTP(ibtp4))
	require.Nil(t, exec.HandleIBTP(ibtp4Receipt))
	meta = exec.QueryLatestMeta()

	v, _ = meta.Load(from)
	require.Equal(t, uint64(4), v.(uint64))

	// test for asset refund ibtp execution
	require.NotNil(t, exec.HandleIBTP(ibtp5))
	require.Nil(t, exec.HandleIBTP(ibtp5Receipt))
	meta = exec.QueryLatestMeta()

	v, _ = meta.Load(from)
	require.Equal(t, uint64(5), v.(uint64))

	// test for wrong type ibtp execution
	require.Nil(t, exec.HandleIBTP(wrongTypeIbtp))
	meta = exec.QueryLatestMeta()

	v, _ = meta.Load(from)
	require.Equal(t, uint64(5), v.(uint64))

	time.Sleep(500 * time.Microsecond)
	require.Nil(t, exec.Stop())
}

func TestQueryReceipt(t *testing.T) {
	exec, _, cli := prepare(t)
	defer exec.storage.Close()

	originalIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN, false)
	cryptoIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN, true)
	cryptoAssetIBTP := getIBTP(t, 1, pb.IBTP_ASSET_EXCHANGE_INIT, true)
	args := [][]byte{[]byte("Alice"), []byte("100")}
	args3 := [][]byte{[]byte("true")}

	cli.EXPECT().GetInMessage(from, uint64(1)).Return(args, nil).AnyTimes()
	cli.EXPECT().GetInMessage(from, uint64(2)).Return(nil, fmt.Errorf("get in message error")).AnyTimes()
	cli.EXPECT().GetInMessage(from, uint64(3)).Return(args3, nil).AnyTimes()
	cli.EXPECT().GetInMessage(to, uint64(4)).Return(args3, nil).AnyTimes()

	// test for get in message error
	receipt, err := exec.QueryReceipt(from, 2, originalIBTP)
	require.NotNil(t, err)

	// test for nil ibtp
	receipt, err = exec.QueryReceipt(from, 1, nil)
	require.NotNil(t, err)

	// test for encrypted right payload but parsing interchain invoke status error
	receipt, err = exec.QueryReceipt(from, 1, cryptoIBTP)
	require.NotNil(t, err)

	// test for encrypted right payload
	receipt, err = exec.QueryReceipt(from, 3, cryptoIBTP)
	require.Nil(t, err)

	// test for encrypted right payload asset IBTP
	receipt, err = exec.QueryReceipt(from, 1, cryptoAssetIBTP)
	require.Nil(t, err)

	// test for not encrypted ibtp
	receipt, err = exec.QueryReceipt(from, 1, originalIBTP)
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
}

func prepare(t *testing.T) (*ChannelExecutor, *mock_agent.MockAgent, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	ag := mock_agent.NewMockAgent(mockCtl)
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

	cli.EXPECT().GetInMeta().Return(make(map[string]uint64), nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(make(map[string]uint64), nil).AnyTimes()
	cryptor.EXPECT().Decrypt(gomock.Any(), from).Return(c, nil).AnyTimes()
	cryptor.EXPECT().Encrypt(gomock.Any(), to).Return(c, nil).AnyTimes()
	exec, err := New(cli, meta.ID, storage, cryptor)
	require.Nil(t, err)
	return exec, ag, cli
}

func getReceipt() *pb.Receipt {
	return &pb.Receipt{
		Version: []byte("0.4.5"),
		TxHash:  &types.Hash{},
		Ret:     nil,
		Status:  0,
	}
}
func getIBTPReceipt(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	receipt := getIBTP(t, index, typ, false)
	receipt.From, receipt.To = receipt.To, receipt.From
	return receipt
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type, isCrypto bool) *pb.IBTP {
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
		Encrypted: isCrypto,
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
