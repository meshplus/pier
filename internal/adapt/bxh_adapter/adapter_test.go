package bxh_adapter

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/cbergoon/merkletree"
	"github.com/golang/mock/gomock"
	service_mgr "github.com/meshplus/bitxhub-core/service-mgr"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	hash               = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	from               = "1356:appchain1:0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	to                 = "1356:appchain2:0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c7196311111111"
	chainId            = "appchain1"
	bxhId1             = 1356
	bxhId2             = 1111
	serviceId1         = "transfer"
	serviceId2         = "data_swapper"
	defaultUnionPierId = "default_union_pier_id"
)

func TestMonitorUpdatedMeta(t *testing.T) {
	adapter, _, _, _ := prepare(t)
	v := adapter.MonitorUpdatedMeta()
	require.Nil(t, v)
}

func TestSendUpdatedMeta(t *testing.T) {
	adapter, _, _, _ := prepare(t)
	v := adapter.SendUpdatedMeta([]byte{})
	require.Nil(t, v)
}

func TestName(t *testing.T) {
	adapter, _, _, _ := prepare(t)
	name := adapter.Name()
	require.Equal(t, fmt.Sprintf("bitxhub:%d", bxhId1), name)
}

func TestStart(t *testing.T) {
	adapter1, _, _, client := prepare(t)
	syncWrapperCh := make(chan interface{}, 1)

	txs1 := make([]*pb.BxhTransaction, 0, 2)
	txs1 = append(txs1, getTx(t, 1, pb.IBTP_INTERCHAIN), getTx(t, 2, pb.IBTP_INTERCHAIN))

	txs2 := make([]*pb.BxhTransaction, 0, 2)
	txs2 = append(txs2, getTx(t, 4, pb.IBTP_INTERCHAIN), getTx(t, 5, pb.IBTP_INTERCHAIN))
	//txCount := len(txs1) + len(txs)

	// mock timeoutIBTP
	timeoutIBTP := &pb.IBTP{
		From:  from,
		To:    to,
		Index: 3,
	}

	multiIBTP := &pb.IBTP{
		From:  from,
		To:    to,
		Index: 6,
		Group: &pb.StringUint64Map{
			Keys: []string{strconv.Itoa(bxhId2)},
			Vals: []uint64{1},
		},
	}

	timeoutTx := &pb.BxhTransaction{
		IBTP: timeoutIBTP,
	}
	timeoutResponse := &pb.GetTransactionResponse{
		Tx: timeoutTx,
	}
	multiTx := &pb.BxhTransaction{
		IBTP: multiIBTP,
	}
	multiResponse := &pb.GetTransactionResponse{
		Tx: multiTx,
	}

	timeoutList := []string{timeoutIBTP.ID()}
	//multiTxList := []string{multiIBTP.ID()}
	w1, _ := getTxWrapper(t, txs1, txs2, []string{timeoutIBTP.ID()}, []string{}, 1, false)
	//w2, _ := getTxWrapper(t, txs2, txs1, []string{}, []string{multiIBTP.ID()}, 2, false)
	syncWrapperCh <- w1

	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetStatus", gomock.Any()).Return(&pb.BxhTransaction{}, nil).AnyTimes()
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(&pb.BxhTransaction{}, nil).AnyTimes()
	client.EXPECT().SendView(gomock.Any()).Return(&pb.Receipt{Ret: []byte("1")}, nil).AnyTimes()
	client.EXPECT().GetTransaction(gomock.Any()).Return(timeoutResponse, nil).Times(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(multiResponse, nil).Times(1)
	client.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).AnyTimes()
	//client.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Receipt{Ret: []byte("1")}, nil).AnyTimes()

	done := make(chan bool)
	go func() {
		err := adapter1.Start()
		require.Nil(t, err)
		<-done
	}()

	ibtpCh := adapter1.MonitorIBTP()

	var (
		ibtps    []*pb.IBTP
		curIndex uint64 = 1
	)
	for i := 1; i <= len(txs1)+1; i++ {
		ibtp := <-ibtpCh
		// timeout ibtp
		if ibtp.To == to {
			require.Equal(t, uint64(3), ibtp.Index)
		} else {
			require.Equal(t, curIndex, ibtp.Index)
			curIndex++
		}
		ibtps = append(ibtps, ibtp)
	}
	require.Equal(t, len(txs1)+len(timeoutList), len(ibtps))
	done <- true

	require.Nil(t, adapter1.Stop())

	//finishStart := make(chan struct{}, 0)
	//ibtpCh2 := make(chan *pb.IBTP, maxChSize)
	//
	//adapter2.ibtpC = nil
	//adapter2.wrappersC = nil
	//adapter2.mode = repo.RelayMode
	//err := adapter2.Start()
	//require.Nil(t, err)
	//ibtpCh2 = adapter2.MonitorIBTP()
	//
	//syncWrapperCh <- w2
	//
	//ibtps = make([]*pb.IBTP, 0)
	//curIndex = uint64(len(txs1) + len(timeoutList) + 1)
	//txCount := len(txs1) + len(timeoutList) + len(txs2) + len(multiTxList)
	//for i := len(txs1) + len(timeoutList) + 1; i <= txCount; i++ {
	//	ibtp := <-ibtpCh2
	//	if ibtp.To == to {
	//		// for multi ibtp
	//		require.Equal(t, uint64(6), ibtp.Index)
	//	} else {
	//		require.Equal(t, curIndex, ibtp.Index)
	//		curIndex++
	//		ibtps = append(ibtps, ibtp)
	//	}
	//}
	//require.Equal(t, len(txs2)+len(multiTxList), len(ibtps))
	//
	//txs3 := make([]*pb.BxhTransaction, 0, 2)
	//txs3 = append(txs3, getTx(t, 1, pb.IBTP_RECEIPT_SUCCESS), getTx(t, 2, pb.IBTP_RECEIPT_FAILURE))
	//
	//txs4 := make([]*pb.BxhTransaction, 0, 2)
	//
	//txs4 = append(txs4, getTxBatch(t, 7), getTxBatch(t, 8))
	//w3, _ := getTxWrapper(t, txs3, txs4, []string{}, []string{}, 3, false)
	//w4, _ := getTxWrapper(t, txs4, txs3, []string{}, []string{}, 4, true)
	//
	//go func() {
	//	syncWrapperCh <- w3
	//	syncWrapperCh <- w4
	//}()
	//
	//ibtps = make([]*pb.IBTP, 0)
	//
	//for i := 1; i <= len(txs3); i++ {
	//	ibtp := <-ibtpCh2
	//	require.Equal(t, uint64(i), ibtp.Index)
	//	ibtps = append(ibtps, ibtp)
	//}
	//
	//for i := 7; i <= 8; i++ {
	//	ibtp := <-ibtpCh2
	//	require.Equal(t, uint64(i), ibtp.Index)
	//	ibtps = append(ibtps, ibtp)
	//}
	//
	//require.Equal(t, len(txs3)+len(txs4), len(ibtps))
	//
	//require.Nil(t, adapter2.Stop())

}

// 4. test broken network when sending ibtp
func TestQueryIBTP(t *testing.T) {
	adapter, _, _, client := prepare(t)

	origin := &pb.IBTP{
		From:  from,
		Index: 1,
	}
	receiptData, err := origin.Marshal()
	require.Nil(t, err)
	normalReceipt := &pb.Receipt{
		Ret:    receiptData,
		Status: pb.Receipt_SUCCESS,
	}

	tmpIP := &pb.InvokePayload{
		Method: "set",
		Args:   []*pb.Arg{{Value: []byte("Alice,10")}},
	}
	pd, err := tmpIP.Marshal()
	require.Nil(t, err)

	td := &pb.TransactionData{
		Payload: pd,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	normalTx := &pb.BxhTransaction{
		IBTP:    origin,
		Payload: data,
	}
	tx := &pb.BxhTransaction{
		Payload: data,
	}
	badReceipt := &pb.Receipt{
		Ret:    []byte("this is a rollback ibtp"),
		Status: pb.Receipt_SUCCESS,
	}
	badReceipt1 := &pb.Receipt{
		Ret:    []byte("this is a failed receipt"),
		Status: pb.Receipt_FAILED,
	}
	normalResponse := &pb.GetTransactionResponse{
		Tx: normalTx,
	}
	//badResponse := &pb.GetTransactionResponse{
	//	Tx: tx,
	//}

	pl := &pb.InvokePayload{
		Method: "GetStatus",
	}

	data1, err := pl.Marshal()
	require.Nil(t, err)

	td1 := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: data1,
	}

	payload1, err := td1.Marshal()
	require.Nil(t, err)

	queryStatusTx := &pb.BxhTransaction{
		To:      constant.TransactionMgrContractAddr.Address(),
		Payload: payload1,
	}

	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).MaxTimes(3)
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(tx, nil).AnyTimes()
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetStatus", gomock.Any()).Return(queryStatusTx, nil).AnyTimes()
	// test for normal receipt
	client.EXPECT().SendView(tx).Return(normalReceipt, nil).Times(1)

	receipt := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_SUCCESS))),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(queryStatusTx).Return(receipt, nil).Times(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil).MaxTimes(1)
	client.EXPECT().GetReceipt(gomock.Any()).Return(normalReceipt, nil).MaxTimes(1)

	//client.EXPECT().InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, gomock.Any()).Return(receipt, nil).AnyTimes()
	ibtp, err := adapter.QueryIBTP(from, true)
	require.Nil(t, err)
	require.Equal(t, origin, ibtp)

	// test for abnormal receipt
	client.EXPECT().SendView(tx).Return(badReceipt, nil).MaxTimes(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 1")).MaxTimes(1)
	ibtp, err = adapter.QueryIBTP(from, true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	// test faied receipt
	client.EXPECT().SendView(tx).Return(badReceipt1, nil).MaxTimes(1)
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetStatus", gomock.Any()).Return(queryStatusTx, nil).AnyTimes()
	receiptStatus := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_BEGIN))),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(queryStatusTx).Return(receiptStatus, nil).MaxTimes(2)

	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil).MaxTimes(1)
	client.EXPECT().GetReceipt(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 2")).MaxTimes(1)
	ibtp, err = adapter.QueryIBTP(from, false)
	require.NotNil(t, err)
	require.Nil(t, ibtp)
}

func TestSendIBTP(t *testing.T) {
	adapter, _, unionAdapter, client := prepare(t)

	b := &types.Address{}
	b.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From: b,
	}

	client.EXPECT().GenerateIBTPTx(gomock.Any()).Return(tx, nil).AnyTimes()

	networkDownTime := 0
	receiptNetworkDownTime := 0
	r := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: pb.Receipt_SUCCESS,
	}
	rf := &pb.Receipt{
		Ret:    []byte("fail receipt"),
		Status: pb.Receipt_FAILED,
	}

	// test union pier succuss send
	client.EXPECT().SendTransactionWithReceipt(gomock.Any(), gomock.Any()).Return(r, nil).Times(1)
	require.Nil(t, unionAdapter.SendIBTP(&pb.IBTP{}))

	client.EXPECT().SendTransactionWithReceipt(gomock.Any(), gomock.Any()).Return(rf, rpcx.ErrRecoverable).Times(5)
	err := adapter.SendIBTP(&pb.IBTP{})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "rollback ibtp")

	client.EXPECT().SendTransactionWithReceipt(gomock.Any(), gomock.Any()).DoAndReturn(
		func(tx *pb.BxhTransaction, opts *rpcx.TransactOpts) (*pb.Receipt, error) {
			networkDownTime++
			if networkDownTime == 1 {
				return nil, rpcx.ErrBrokenNetwork
			} else if networkDownTime == 2 {
				return nil, rpcx.ErrReconstruct
			}
			receiptNetworkDownTime++
			if receiptNetworkDownTime == 1 {
				return rf, fmt.Errorf("not found receipt")
			}
			return r, nil
		}).MaxTimes(4)

	require.Nil(t, adapter.SendIBTP(&pb.IBTP{}))

	receipt := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_SUCCESS))),
		Status: pb.Receipt_SUCCESS,
	}

	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).AnyTimes()
	client.EXPECT().InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, gomock.Any()).Return(receipt, nil).AnyTimes()

	//err := adapter.RegisterRollbackHandler(func(ibtp *pb.IBTP, ibtpId string) {})
	//require.Nil(t, err)

	// test for receipt failed situation
	errMsg1 := fmt.Sprintf("%s: appchain not registerd", noBindRule)
	//errMsg2 := fmt.Sprintf("%s: current appchain err", CurAppchainNotAvailable)
	errMsg3 := fmt.Sprintf("%s: source service err", SrcServiceNotAvailable)
	//errMsg4 := fmt.Sprintf("%s: target service err", InvalidTargetService)

	//errMsg5 := fmt.Sprintf("%s: target appchain err", TargetAppchainNotAvailable)
	//errMsg6 := fmt.Sprintf("%s: target service err", TargetServiceNotAvailable)
	//errMsg7 := fmt.Sprintf("%s: target bitxhub err", TargetBitXHubNotAvailable)

	errMsg8 := fmt.Sprintf("%s: ibtp already on chain", ibtpIndexExist)
	errMsg9 := fmt.Sprintf("%s: ibtp index is too high", ibtpIndexWrong)
	errMsg10 := fmt.Sprintf("%s: ibtp verify error", invalidIBTP)
	errMsg11 := fmt.Sprintf("%s: ibtp verify failed", proofFailed)
	errMsg12 := fmt.Sprintf("state BEGIN_ROLLBACK get unexpected receipt")
	failReceipt := &pb.Receipt{
		Ret:    []byte(errMsg1),
		Status: pb.Receipt_FAILED,
	}
	client.EXPECT().SendTransactionWithReceipt(gomock.Any(), gomock.Any()).Return(failReceipt, nil).AnyTimes()
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr1, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.ValidationRulesUnregister, ibtpErr1.Status)

	//failReceipt.Ret = []byte(errMsg2)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//ibtpErr2, ok := err.(*adapt.SendIbtpError)
	//require.Equal(t, true, ok)
	//require.Equal(t, adapt.SrcChain_Unavailable, ibtpErr2.Status)

	failReceipt.Ret = []byte(errMsg3)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr3, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.SrcChainServiceUnavailable, ibtpErr3.Status)

	//failReceipt.Ret = []byte(errMsg4)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//require.Nil(t, err)

	// if target chain is not available, this ibtp should be rollback
	//failReceipt.Ret = []byte(errMsg5)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//require.Nil(t, err)
	//<-adapter.ibtpC

	//failReceipt.Ret = []byte(errMsg6)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//require.Nil(t, err)
	//<-adapter.ibtpC

	//failReceipt.Ret = []byte(errMsg7)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//require.Nil(t, err)
	//<-adapter.ibtpC

	failReceipt.Ret = []byte(errMsg8)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)

	failReceipt.Ret = []byte(errMsg9)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr9, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.IndexWrong, ibtpErr9.Status)

	failReceipt.Ret = []byte(errMsg10)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr10, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.InvalidIBTP, ibtpErr10.Status)

	failReceipt.Ret = []byte(errMsg11)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr11, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.ProofInvalid, ibtpErr11.Status)

	// test rollback IBTP
	tmpPd := &pb.InvokePayload{
		Method: "set",
		Args:   []*pb.Arg{{Value: []byte("Alice,10")}},
	}
	pd, err := tmpPd.Marshal()
	require.Nil(t, err)

	td := &pb.TransactionData{
		Payload: pd,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	bxhProof := &pb.BxhProof{
		TxStatus:  pb.TransactionStatus_BEGIN_ROLLBACK,
		MultiSign: make([][]byte, 0),
	}
	proof, err := bxhProof.Marshal()
	require.Nil(t, err)

	response := make(map[string][]byte)
	response[hash] = proof

	signResponse := &pb.SignResponse{
		Sign: response,
	}

	ibtp := &pb.IBTP{
		From:    from,
		Index:   1,
		Type:    pb.IBTP_INTERCHAIN,
		Payload: data,
	}
	receiptData, err := ibtp.Marshal()
	require.Nil(t, err)

	rollReceipt := &pb.Receipt{
		Ret:    receiptData,
		Status: pb.Receipt_SUCCESS,
	}

	failReceipt.Ret = []byte(errMsg12)
	rollbackTx := &pb.BxhTransaction{
		From: b,
		IBTP: ibtp,
	}
	rollbackResponse := &pb.GetTransactionResponse{
		Tx: rollbackTx,
	}

	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(signResponse, nil).MaxTimes(1)
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
	client.EXPECT().SendView(gomock.Any()).Return(rollReceipt, nil).MaxTimes(1)

	pl := &pb.InvokePayload{
		Method: "GetStatus",
	}

	data1, err := pl.Marshal()
	require.Nil(t, err)

	td1 := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: data1,
	}

	payload1, err := td1.Marshal()
	require.Nil(t, err)

	queryStatusTx := &pb.BxhTransaction{
		To:      constant.TransactionMgrContractAddr.Address(),
		Payload: payload1,
	}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetStatus", gomock.Any()).Return(queryStatusTx, nil).AnyTimes()
	receiptStatus := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_BEGIN_ROLLBACK))),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(queryStatusTx).Return(receiptStatus, nil).MaxTimes(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(rollbackResponse, nil).MaxTimes(1)

	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
}

func TestGetServiceIDList(t *testing.T) {
	relayAdapter, _, unionAdapter, client := prepare(t)
	// get serviceID
	services := getServiceID(t)
	data1, err := json.Marshal(services)
	require.Nil(t, err)
	reciept1 := &pb.Receipt{
		Ret:    data1,
		Status: pb.Receipt_SUCCESS,
	}

	failData := "interchain info does not exist"
	failReceipt := &pb.Receipt{
		Ret:    []byte(failData),
		Status: pb.Receipt_FAILED,
	}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).Return(&pb.BxhTransaction{}, nil).Times(2)

	client.EXPECT().SendView(gomock.Any()).Return(nil, fmt.Errorf("test sendView err")).Times(1)
	client.EXPECT().SendView(gomock.Any()).Return(failReceipt, nil).Times(1)
	client.EXPECT().SendView(gomock.Any()).Return(reciept1, nil).Times(1)

	loggers.InitializeLogger(repo.DefaultConfig())
	// test fail and wrong receipt
	ids, err := relayAdapter.GetServiceIDList()
	require.Nil(t, ids)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "info is not exit in bitxhub")

	ids1, err := relayAdapter.GetServiceIDList()
	require.Nil(t, err)
	require.Equal(t, 2, len(ids1))
	require.Equal(t, serviceId1, ids1[0])
	require.Equal(t, serviceId2, ids1[1])

	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(),
		gomock.Any()).Return(&pb.BxhTransaction{}, nil).Times(2)

	services2 := make([]string, 0)
	services2 = append(services2, fmt.Sprintf("%s:%s:%s", strconv.Itoa(bxhId1), chainId, serviceId1),
		fmt.Sprintf("%s:%s:%s", strconv.Itoa(bxhId2), chainId, serviceId2))
	data2, err := json.Marshal(services2)
	reciept2 := &pb.Receipt{
		Ret:    data2,
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(gomock.Any()).Return(reciept2, nil).Times(1)

	// get registered bxhIds in Union Mode
	bxhIds := make([]string, 0)
	bxhIds = append(bxhIds, strconv.Itoa(bxhId1), strconv.Itoa(bxhId2))
	data3, err := json.Marshal(bxhIds)
	reciept3 := &pb.Receipt{
		Ret:    data3,
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(gomock.Any()).Return(reciept3, nil).Times(1)
	ids2, err := unionAdapter.GetServiceIDList()
	require.Nil(t, err)
	require.Equal(t, 1, len(ids2))
}

func TestQueryInterchain(t *testing.T) {
	adapter, _, _, client := prepare(t)

	b := &types.Address{}
	b.SetBytes([]byte(from))
	queryTx := &pb.BxhTransaction{
		From: b,
	}

	originalMeta := &pb.Interchain{
		InterchainCounter: map[string]uint64{from: 2},
	}
	metaData, err := originalMeta.Marshal()
	require.Nil(t, err)
	normalReceipt := &pb.Receipt{
		Ret:    metaData,
		Status: 0,
	}
	badReceipt := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: pb.Receipt_FAILED,
	}
	badReceipt1 := &pb.Receipt{
		Ret:    []byte("this is a failed receipt"),
		Status: pb.Receipt_FAILED,
	}
	emptyReceipt := &pb.Receipt{
		Ret:    []byte{},
		Status: pb.Receipt_SUCCESS,
	}

	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetInterchain", gomock.Any()).Return(queryTx, nil).AnyTimes()
	client.EXPECT().SendView(queryTx).Return(badReceipt, nil)
	client.EXPECT().SendView(queryTx).Return(badReceipt1, nil)
	client.EXPECT().SendView(queryTx).Return(emptyReceipt, nil)
	loggers.InitializeLogger(repo.DefaultConfig())
	// test empty receipt
	meta, err := adapter.QueryInterchain(from)
	require.Nil(t, err)
	require.Equal(t, 0, len(meta.SourceInterchainCounter))

	// test normal receipt
	client.EXPECT().SendView(queryTx).Return(normalReceipt, nil)
	meta, err = adapter.QueryInterchain(from)
	require.Nil(t, err)
	require.Equal(t, originalMeta.InterchainCounter[from], meta.InterchainCounter[from])
}

func prepare(t *testing.T) (*BxhAdapter, *BxhAdapter, *BxhAdapter, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	client := mock_client.NewMockClient(mockCtl)
	client.EXPECT().GetChainID().Return(uint64(bxhId1), nil).AnyTimes()

	config := &repo.Config{}
	config.Mode.Type = repo.RelayMode
	config.TSS = &repo.TSS{
		EnableTSS: false,
	}

	relayAdapter1, err := New(repo.RelayMode, "",
		client, log.NewWithModule("adapter"), config.TSS, 0,
	)
	require.Nil(t, err)

	relayAdapter2, err := New(repo.RelayMode, "",
		client, log.NewWithModule("adapter"), config.TSS, 0,
	)
	require.Nil(t, err)

	unionAdapter, err := New(repo.UnionMode, defaultUnionPierId,
		client, log.NewWithModule("adapter"), config.TSS, 0,
	)
	require.Nil(t, err)

	return relayAdapter1, relayAdapter2, unionAdapter, client
}

func getBlockHeader(root *types.Hash, number uint64) *pb.BlockHeader {
	wrapper := &pb.BlockHeader{
		Number:      number,
		Timestamp:   time.Now().UnixNano(),
		ParentHash:  types.NewHashByStr(from),
		TxRoot:      root,
		TimeoutRoot: &types.Hash{},
	}

	return wrapper
}

func getTxWrapper(t *testing.T, interchainTxs []*pb.BxhTransaction, innerchainTxs []*pb.BxhTransaction, timeoutList, multiTxList []string, number uint64, isBatch bool) (*pb.InterchainTxWrappers, *types.Hash) {
	verifiedTxs := make([]*pb.VerifiedTx, len(interchainTxs))
	for i, tx := range interchainTxs {
		verifiedTxs[i] = &pb.VerifiedTx{
			Tx:      tx,
			Valid:   true,
			IsBatch: isBatch,
		}
	}
	var l2roots []types.Hash

	// cal interchain root
	hashes := make([]merkletree.Content, 0, len(interchainTxs))
	for i := 0; i < len(verifiedTxs); i++ {
		//interchainHash, err := verifiedTxs[i].CalculateHash()
		//require.Nil(t, err)
		hashes = append(hashes, verifiedTxs[i])
		//interchainTxHashes = append(interchainTxHashes, *interchainHash)
	}

	tree, err := merkletree.NewTree(hashes)
	require.Nil(t, err)
	l2roots = append(l2roots, *types.NewHash(tree.MerkleRoot()))

	// cal innerchain root
	hashes = make([]merkletree.Content, 0, len(innerchainTxs))
	for i := 0; i < len(innerchainTxs); i++ {
		hashes = append(hashes, innerchainTxs[i].Hash())
	}
	tree, _ = merkletree.NewTree(hashes)
	l2roots = append(l2roots, *types.NewHash(tree.MerkleRoot()))

	contents := make([]merkletree.Content, 0, len(l2roots))
	for _, root := range l2roots {
		r := root
		contents = append(contents, &r)
	}
	tree, _ = merkletree.NewTree(contents)
	l1root := tree.MerkleRoot()

	var timeoutRoot types.Hash
	if len(timeoutList) > 0 {
		timeoutRoot, err = calcTimeoutL2Root(timeoutList)
		require.Nil(t, err)
	}
	var timeoutL2Roots []types.Hash
	timeoutL2Roots = append(timeoutL2Roots, timeoutRoot)
	wrappers := make([]*pb.InterchainTxWrapper, 0, 1)
	wrapper := &pb.InterchainTxWrapper{
		Transactions:   verifiedTxs,
		Height:         number,
		L2Roots:        l2roots,
		TimeoutL2Roots: timeoutL2Roots,
		TimeoutIbtps:   timeoutList,
		MultiTxIbtps:   multiTxList,
	}
	wrappers = append(wrappers, wrapper)
	itw := &pb.InterchainTxWrappers{
		InterchainTxWrappers: wrappers,
	}
	return itw, types.NewHash(l1root)
}

func calcTimeoutL2Root(list []string) (types.Hash, error) {
	hashes := make([]merkletree.Content, 0, len(list))
	for _, id := range list {
		hash := sha256.Sum256([]byte(id))
		hashes = append(hashes, types.NewHash(hash[:]))
	}

	tree, err := merkletree.NewTree(hashes)
	if err != nil {
		return types.Hash{}, fmt.Errorf("init merkle tree: %w", err)
	}

	return *types.NewHash(tree.MerkleRoot()), nil
}

func getTx(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.BxhTransaction {
	ibtp := getIBTP(t, index, typ)
	body, err := ibtp.Marshal()
	require.Nil(t, err)

	tmpIP := &pb.InvokePayload{
		Method: "set",
		Args:   []*pb.Arg{{Value: body}},
	}
	pd, err := tmpIP.Marshal()
	require.Nil(t, err)

	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: pd,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From:    faddr,
		To:      faddr,
		IBTP:    ibtp,
		Payload: data,
	}
	tx.TransactionHash = tx.Hash()
	return tx
}

func getTxBatch(t *testing.T, index uint64) *pb.BxhTransaction {
	ibtp := getIBTP(t, index, pb.IBTP_INTERCHAIN)
	ibtp.Extra = []byte("1")

	tmpIP := &pb.Payload{
		Encrypted: false,
		Content:   []byte("test"),
		Hash:      types.NewHash([]byte("test")).Bytes(),
	}
	pd, err := tmpIP.Marshal()
	require.Nil(t, err)

	td := &pb.TransactionData{
		Payload: pd,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From:    faddr,
		To:      faddr,
		IBTP:    ibtp,
		Payload: data,
	}
	tx.TransactionHash = tx.Hash()
	return tx
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	ct := &pb.Content{
		Func: "set",
		Args: [][]byte{[]byte("Alice")},
	}
	c, err := ct.Marshal()
	require.Nil(t, err)

	pd := pb.Payload{
		Encrypted: false,
		Content:   c,
	}
	ibtppd, err := pd.Marshal()
	require.Nil(t, err)

	return &pb.IBTP{
		From:      from,
		To:        from,
		Payload:   ibtppd,
		Index:     index,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
	}
}

func getServiceID(t *testing.T) []*service_mgr.Service {
	services := make([]*service_mgr.Service, 0)
	service1 := &service_mgr.Service{
		ServiceID: serviceId1,
	}
	service2 := &service_mgr.Service{
		ServiceID: serviceId2,
	}
	services = append(services, service1, service2)
	require.Equal(t, 2, len(services))

	return services
}
