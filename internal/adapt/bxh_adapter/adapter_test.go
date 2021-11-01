package bxh_adapter

import (
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
	from               = "did:bitxhub:testappchain:."
	chainId            = "testAppchain"
	bxhId1             = 1356
	bxhId2             = 1111
	serviceId1         = "transfer"
	serviceId2         = "data_swapper"
	defaultUnionPierId = "default_union_pier_id"
)

func TestName(t *testing.T) {
	adapter, _, _ := prepare(t)
	name := adapter.Name()
	require.Equal(t, fmt.Sprintf("bitxhub:%d", bxhId1), name)
}

func TestStart(t *testing.T) {
	adapter, _, client := prepare(t)
	syncWrapperCh := make(chan interface{}, 4)

	txs := make([]*pb.BxhTransaction, 0, 2)
	txs = append(txs, getTx(t, 1), getTx(t, 2))

	txs1 := make([]*pb.BxhTransaction, 0, 2)
	txs1 = append(txs1, getTx(t, 3), getTx(t, 4))
	txCount := len(txs1) + len(txs)

	w1, _ := getTxWrapper(t, txs, txs1, 1)
	w2, _ := getTxWrapper(t, txs1, txs, 2)
	go func() {
		syncWrapperCh <- w1
		syncWrapperCh <- w2
	}()

	client.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).AnyTimes()
	client.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Receipt{Ret: []byte("1")}, nil).AnyTimes()

	done := make(chan bool)
	go func() {
		err := adapter.Start()
		require.Nil(t, err)
		<-done
	}()

	ibtpCh := adapter.MonitorIBTP()

	var ibtps []*pb.IBTP
	for i := 1; i <= txCount; i++ {
		ibtp := <-ibtpCh
		require.Equal(t, uint64(i), ibtp.Index)
		ibtps = append(ibtps, ibtp)
	}
	require.Equal(t, txCount, len(ibtps))
	done <- true

	require.Nil(t, adapter.Stop())
}

// 4. test broken network when sending ibtp
func TestQueryIBTP(t *testing.T) {
	adapter, _, client := prepare(t)

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
	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).MaxTimes(3)
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(tx, nil).AnyTimes()
	// test for normal receipt
	client.EXPECT().SendView(tx).Return(normalReceipt, nil).MaxTimes(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil).MaxTimes(1)
	client.EXPECT().GetReceipt(gomock.Any()).Return(normalReceipt, nil).MaxTimes(1)

	receipt := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_SUCCESS))),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, gomock.Any()).Return(receipt, nil).AnyTimes()
	ibtp, err := adapter.QueryIBTP(from, true)
	require.Nil(t, err)
	require.Equal(t, origin, ibtp)

	// test for abnormal receipt
	client.EXPECT().SendView(tx).Return(badReceipt, nil).MaxTimes(1)
	client.EXPECT().GetTransaction(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 1")).MaxTimes(1)
	ibtp, err = adapter.QueryIBTP(from, true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	client.EXPECT().SendView(tx).Return(badReceipt1, nil).MaxTimes(2)
	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil).MaxTimes(2)
	client.EXPECT().GetReceipt(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 2")).MaxTimes(1)
	ibtp, err = adapter.QueryIBTP(from, false)
	require.NotNil(t, err)
	require.Nil(t, ibtp)
}

func TestSendIBTP(t *testing.T) {
	adapter, _, client := prepare(t)

	b := &types.Address{}
	b.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From: b,
	}

	client.EXPECT().GenerateIBTPTx(gomock.Any()).Return(tx, nil).AnyTimes()

	networkDownTime := 0
	client.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(tx *pb.BxhTransaction, opts *rpcx.TransactOpts) (string, error) {
			networkDownTime++
			if networkDownTime == 1 {
				return "", rpcx.ErrBrokenNetwork
			} else if networkDownTime == 2 {
				return "", rpcx.ErrReconstruct
			}
			return hash, nil
		}).MaxTimes(4)

	r := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: pb.Receipt_SUCCESS,
	}
	receiptNetworkDownTime := 0
	client.EXPECT().GetReceipt(hash).DoAndReturn(
		func(ha string) (*pb.Receipt, error) {
			receiptNetworkDownTime++
			if receiptNetworkDownTime == 1 {
				return nil, fmt.Errorf("not found receipt")
			}
			return r, nil
		}).MaxTimes(2)
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
	client.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).Return(hash, nil).AnyTimes()
	errMsg1 := fmt.Sprintf("%s: appchain not registerd", noBindRule)
	errMsg2 := fmt.Sprintf("%s: current appchain err", CurAppchainNotAvailable)
	errMsg3 := fmt.Sprintf("%s: current service err", CurServiceNotAvailable)
	errMsg4 := fmt.Sprintf("%s: target service err", InvalidTargetService)

	errMsg5 := fmt.Sprintf("%s: target appchain err", TargetAppchainNotAvailable)
	errMsg6 := fmt.Sprintf("%s: target service err", TargetServiceNotAvailable)
	errMsg7 := fmt.Sprintf("%s: target bitxhub err", TargetBitXHubNotAvailable)

	errMsg8 := fmt.Sprintf("%s: ibtp already on chain", ibtpIndexExist)
	errMsg9 := fmt.Sprintf("%s: ibtp index is too high", ibtpIndexWrong)
	errMsg10 := fmt.Sprintf("%s: ibtp verify error", invalidIBTP)
	errMsg11 := fmt.Sprintf("%s: ibtp verify failed", proofFailed)
	errMsg12 := fmt.Sprintf("state BEGIN_ROLLBACK get unexpected receipt")
	failReceipt := &pb.Receipt{
		Ret:    []byte(errMsg1),
		Status: pb.Receipt_FAILED,
	}
	client.EXPECT().GetReceipt(hash).Return(failReceipt, nil).AnyTimes()
	err := adapter.SendIBTP(&pb.IBTP{})
	ibtpErr1, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.ValidationRules_Unregister, ibtpErr1.Status)

	failReceipt.Ret = []byte(errMsg2)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr2, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.SrcChain_Unavailable, ibtpErr2.Status)

	failReceipt.Ret = []byte(errMsg3)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr3, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.SrcChainService_Unavailable, ibtpErr3.Status)

	failReceipt.Ret = []byte(errMsg4)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)

	// if target chain is not available, this ibtp should be rollback
	failReceipt.Ret = []byte(errMsg5)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
	<-adapter.ibtpC

	failReceipt.Ret = []byte(errMsg6)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
	<-adapter.ibtpC

	failReceipt.Ret = []byte(errMsg7)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
	<-adapter.ibtpC

	failReceipt.Ret = []byte(errMsg8)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)

	failReceipt.Ret = []byte(errMsg9)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr9, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.Index_Wrong, ibtpErr9.Status)

	failReceipt.Ret = []byte(errMsg10)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr10, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.Other_Error, ibtpErr10.Status)

	failReceipt.Ret = []byte(errMsg11)
	err = adapter.SendIBTP(&pb.IBTP{})
	ibtpErr11, ok := err.(*adapt.SendIbtpError)
	require.Equal(t, true, ok)
	require.Equal(t, adapt.Proof_Invalid, ibtpErr11.Status)

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
	client.EXPECT().GetTransaction(gomock.Any()).Return(rollbackResponse, nil).MaxTimes(1)

	err = adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
}

func TestGetServiceIDList(t *testing.T) {
	relayAdapter, unionAdapter, client := prepare(t)
	// get serviceID
	services := getServiceID(t)
	data1, err := json.Marshal(services)
	require.Nil(t, err)
	reciept1 := &pb.Receipt{
		Ret:    data1,
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(),
		gomock.Any()).Return(&pb.BxhTransaction{}, nil).MaxTimes(2)
	client.EXPECT().SendView(gomock.Any()).Return(reciept1, nil).MaxTimes(1)

	// get registered bxhIds in Union Mode
	bxhIds := make([]string, 0)
	bxhIds = append(bxhIds, strconv.Itoa(bxhId1), strconv.Itoa(bxhId2))
	data2, err := json.Marshal(bxhIds)
	reciept2 := &pb.Receipt{
		Ret:    data2,
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().SendView(gomock.Any()).Return(reciept2, nil).MaxTimes(1)

	loggers.InitializeLogger(repo.DefaultConfig())
	ids1, err := relayAdapter.GetServiceIDList()
	require.Nil(t, err)
	require.Equal(t, 0, len(ids1))

	ids2, err := unionAdapter.GetServiceIDList()
	require.Nil(t, err)
	require.Equal(t, 1, len(ids2))
}

func TestQueryInterchain(t *testing.T) {
	adapter, _, client := prepare(t)

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

func prepare(t *testing.T) (*BxhAdapter, *BxhAdapter, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	client := mock_client.NewMockClient(mockCtl)
	client.EXPECT().GetChainID().Return(uint64(bxhId1), nil).AnyTimes()

	config := &repo.Config{}
	config.Mode.Type = repo.RelayMode

	relayAdapter, err := New(repo.RelayMode, "",
		client, log.NewWithModule("adapter"),
	)
	require.Nil(t, err)

	unionAdapter, err := New(repo.UnionMode, defaultUnionPierId,
		client, log.NewWithModule("adapter"),
	)
	require.Nil(t, err)

	return relayAdapter, unionAdapter, client
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

func getTxWrapper(t *testing.T, interchainTxs []*pb.BxhTransaction, innerchainTxs []*pb.BxhTransaction, number uint64) (*pb.InterchainTxWrappers, *types.Hash) {
	verifiedTxs := make([]*pb.VerifiedTx, len(interchainTxs))
	for i, tx := range interchainTxs {
		verifiedTxs[i] = &pb.VerifiedTx{
			Tx:    tx,
			Valid: true,
		}
	}
	var l2roots []types.Hash
	//var interchainTxHashes []types.Hash
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

	wrappers := make([]*pb.InterchainTxWrapper, 0, 1)
	wrapper := &pb.InterchainTxWrapper{
		Transactions: verifiedTxs,
		Height:       number,
		L2Roots:      l2roots,
	}
	wrappers = append(wrappers, wrapper)
	itw := &pb.InterchainTxWrappers{
		InterchainTxWrappers: wrappers,
	}
	return itw, types.NewHash(l1root)
}

func getTx(t *testing.T, index uint64) *pb.BxhTransaction {
	ibtp := getIBTP(t, index, pb.IBTP_INTERCHAIN)
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
		From:    from,
		To:      from,
		Payload: ibtppd,
		Index:   index,
		Type:    typ,
	}
}

func getServiceID(t *testing.T) []string {
	services := make([]string, 0)
	service1 := &service_mgr.Service{
		ServiceID: fmt.Sprintf("%s:%s:%s", strconv.Itoa(bxhId1), chainId, serviceId1),
	}
	service2 := &service_mgr.Service{
		ServiceID: fmt.Sprintf("%s:%s:%s", strconv.Itoa(bxhId2), chainId, serviceId2),
	}
	services = append(services, service1.ServiceID, service2.ServiceID)
	require.Equal(t, 2, len(services))

	return services
}
