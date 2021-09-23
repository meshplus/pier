package bxh_adapter

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/meshplus/bitxhub-model/constant"

	"github.com/meshplus/pier/internal/loggers"

	"github.com/cbergoon/merkletree"
	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	appchainMethod = "did:bitxhub:etherappchain:."
	hash           = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	from           = "did:bitxhub:testappchain:."
)

//// 1. test normal interchain wrapper
//// 2. test interchain wrappers arriving at wrong sequence
//// 3. test invalid interchain wrappers
//func TestSyncHeader001(t *testing.T) {
//	adapter, client := prepare(t)
//	ctx, cancel := context.WithCancel(context.Background())
//	adapter.ctx = ctx
//	adapter.cancel = cancel
//
//	// expect mock module returns
//	txs := make([]*pb.BxhTransaction, 0, 2)
//	txs = append(txs, getTx(t), getTx(t))
//
//	txs1 := make([]*pb.BxhTransaction, 0, 2)
//	txs1 = append(txs1, getTx(t), getTx(t))
//
//	w1, _ := getTxWrapper(t, txs, txs1, 1)
//	w2, _ := getTxWrapper(t, txs, txs1, 2)
//	w3, root := getTxWrapper(t, txs, txs1, 3)
//	h3 := getBlockHeader(root, 3)
//	// mock invalid tx wrapper
//	w4, _ := getTxWrapper(t, txs, txs1, 4)
//	w4.InterchainTxWrappers[0].Transactions = []*pb.VerifiedTx{{
//		Tx:    getTx(t),
//		Valid: false,
//	}}
//	// mock nil wrapper
//	w5 := &pb.InterchainTxWrappers{}
//	// mock wrong height wrapper
//	w6, _ := getTxWrapper(t, txs, txs1, 1)
//	w7 := &pb.InterchainTxWrappers{InterchainTxWrappers: []*pb.InterchainTxWrapper{nil}}
//	// mock bad tx wrapper
//	badTxs := make([]*pb.BxhTransaction, 0, 2)
//	badTxs = append(badTxs, getUnoinTx(t), getUnoinTx(t))
//
//	badTxs2 := make([]*pb.BxhTransaction, 0, 2)
//	badTxs2 = append(badTxs2, getUnoinTx(t), getUnoinTx(t))
//	w8, _ := getTxWrapper(t, badTxs, badTxs2, 4)
//
//	syncWrapperCh := make(chan interface{}, 4)
//	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER, gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
//
//	go func() {
//		syncWrapperCh <- w3
//		syncWrapperCh <- w4
//		syncWrapperCh <- w5
//		syncWrapperCh <- w6
//		syncWrapperCh <- w7
//		syncWrapperCh <- w8
//	}()
//
//	done := make(chan bool, 1)
//	go func() {
//		err := adapter.Start()
//		require.Nil(t, err)
//		<-done
//		require.Nil(t, adapter.Stop())
//	}()
//
//	time.Sleep(1 * time.Second)
//}
//
//func TestSyncUnoinHeader(t *testing.T) {
//	adapter, _ := prepare(t)
//	ctx, cancel := context.WithCancel(context.Background())
//	adapter.ctx = ctx
//	adapter.cancel = cancel
//
//	// expect mock module returns
//	txs := make([]*pb.BxhTransaction, 0, 2)
//	txs = append(txs, getUnoinTx(t), getUnoinTx(t))
//
//	txs1 := make([]*pb.BxhTransaction, 0, 2)
//	txs1 = append(txs1, getUnoinTx(t), getUnoinTx(t))
//
//	w1, _ := getTxWrapper(t, txs, txs1, 1)
//	w2, root := getTxWrapper(t, txs, txs1, 2)
//	h2 := getBlockHeader(root, 2)
//
//	lite.EXPECT().QueryHeader(gomock.Any()).Return(h2, nil).AnyTimes()
//
//	adapter.isRecover = true
//	adapter.mode = repo.UnionMode
//	icm := make(map[string]*rpcx.Interchain)
//	// recover for error and normal situation
//	adapter.handleInterchainWrapperAndPersist(w1, icm)
//	adapter.handleInterchainWrapperAndPersist(w1, icm)
//	adapter.height = 1
//	adapter.handleInterchainWrapperAndPersist(w2, icm)
//
//	time.Sleep(1 * time.Second)
//	require.Equal(t, uint64(2), adapter.height)
//}
//
//func recoverFail(_ *pb.IBTP) (*rpcx.Interchain, error) {
//	return nil, fmt.Errorf("recover interchain meta failed")
//}
//
//func recoverHandler(ibtp *pb.IBTP) (*rpcx.Interchain, error) {
//	return &rpcx.Interchain{
//		ID:                   from,
//		InterchainCounter:    map[string]uint64{ibtp.To: 1},
//		SourceReceiptCounter: map[string]uint64{},
//		ReceiptCounter:       map[string]uint64{},
//	}, nil
//}
//
//func TestSyncHeader002(t *testing.T) {
//	adapter, client, lite := prepare(t, 1)
//	defer adapter.storage.Close()
//
//	// expect mock module returns
//	txs := make([]*pb.BxhTransaction, 0, 2)
//	txs = append(txs, getTx(t), getTx(t))
//
//	txs1 := make([]*pb.BxhTransaction, 0, 2)
//	txs1 = append(txs1, getTx(t), getTx(t))
//
//	w2, _ := getTxWrapper(t, txs, txs1, 2)
//	w3, root := getTxWrapper(t, txs, txs1, 3)
//	h3 := getBlockHeader(root, 3)
//	// mock invalid tx wrapper
//	w4, _ := getTxWrapper(t, txs, txs1, 4)
//	//w4.InterchainTxWrappers[0].Transactions = w4.InterchainTxWrappers[0].TransactionHashes[1:]
//
//	syncWrapperCh := make(chan interface{}, 4)
//	meta := &pb.ChainMeta{
//		Height:    1,
//		BlockHash: types.NewHashByStr(from),
//	}
//	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER, gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
//	client.EXPECT().GetInterchainTxWrappers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
//		func(ctx context.Context, pid string, begin, end uint64, ch chan<- *pb.InterchainTxWrappers) {
//			if begin == 2 {
//				ch <- w2
//			} else if begin == 3 {
//				ch <- w3
//			}
//			close(ch)
//		}).AnyTimes()
//	client.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
//	lite.EXPECT().QueryHeader(gomock.Any()).Return(h3, nil).AnyTimes()
//	go func() {
//		syncWrapperCh <- w3
//		syncWrapperCh <- w4
//	}()
//
//	adapter.height = 1
//	ok, err := adapter.verifyWrapper(w2.InterchainTxWrappers[0])
//	require.Nil(t, err)
//	require.Equal(t, true, ok)
//
//	done := make(chan bool, 1)
//	go func() {
//		err := adapter.Start()
//		require.Nil(t, err)
//		<-done
//	}()
//
//	time.Sleep(1 * time.Second)
//
//	// recover should have persist height 3 wrapper
//	receiveWrapper := &pb.InterchainTxWrappers{}
//	val := adapter.storage.Get(model.WrapperKey(3))
//
//	require.Nil(t, receiveWrapper.Unmarshal(val))
//	for i, tx := range w3.InterchainTxWrappers[0].Transactions {
//		require.Equal(t, tx.Tx.TransactionHash.String(), receiveWrapper.InterchainTxWrappers[0].Transactions[i].Tx.TransactionHash.String())
//	}
//	done <- true
//	require.Equal(t, uint64(3), adapter.height)
//	require.Nil(t, adapter.Stop())
//}

func TestStart(t *testing.T) {
	adapter, _ := prepare(t)
	err := adapter.Start()
	require.Nil(t, err)
}

// 4. test broken network when sending ibtp
func TestQueryIBTP(t *testing.T) {
	adapter, client := prepare(t)

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
		Ret:    []byte("this is a test"),
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
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(tx, nil).AnyTimes()
	// test for normal receipt
	client.EXPECT().SendView(tx).Return(normalReceipt, nil)
	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil)
	client.EXPECT().GetReceipt(gomock.Any()).Return(normalReceipt, nil)

	receipt := &pb.Receipt{
		Ret:    []byte(strconv.Itoa(int(pb.TransactionStatus_SUCCESS))),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, gomock.Any()).Return(receipt, nil).AnyTimes()
	ibtp, err := adapter.QueryIBTP(from, true)
	require.Nil(t, err)
	require.Equal(t, origin, ibtp)

	// test for abnormal receipt
	client.EXPECT().SendView(tx).Return(badReceipt, nil)
	client.EXPECT().GetTransaction(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 1"))
	ibtp, err = adapter.QueryIBTP(from, true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	client.EXPECT().SendView(tx).Return(badReceipt1, nil)
	client.EXPECT().GetTransaction(gomock.Any()).Return(normalResponse, nil)
	client.EXPECT().GetReceipt(gomock.Any()).Return(nil, fmt.Errorf("bad ibtp 2"))
	ibtp, err = adapter.QueryIBTP(from, true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)
}

//func TestGetIBTPSigns(t *testing.T) {
//	adapter, client := prepare(t)
//
//	ibtp := &pb.IBTP{}
//	hash := ibtp.Hash().String()
//
//	digest := sha256.Sum256([]byte(hash))
//	priv, err := asym.GenerateKeyPair(crypto.Secp256k1)
//	require.Nil(t, err)
//	sig, err := priv.Sign(digest[:])
//	require.Nil(t, err)
//	resp := &pb.SignResponse{
//		Sign: map[string][]byte{hash: sig},
//	}
//
//	// test for normal response
//	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
//		Return(resp, nil)
//	sigBytes, err := adapter.GetIBTPSigns(ibtp)
//	require.Nil(t, err)
//	require.Equal(t, true, bytes.Equal(sigBytes, sigBytes))
//
//	// test for abnormal receipt
//	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
//		Return(nil, nil)
//	sigBytes, err = adapter.GetIBTPSigns(ibtp)
//	require.NotNil(t, err)
//	require.Nil(t, sigBytes)
//
//	resp.Sign = nil
//	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
//		Return(resp, nil)
//	sigBytes, err = adapter.GetIBTPSigns(ibtp)
//	require.NotNil(t, err)
//	require.Nil(t, sigBytes)
//
//	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
//		Return(nil, fmt.Errorf("test error"))
//	sigBytes, err = adapter.GetIBTPSigns(ibtp)
//	require.NotNil(t, err)
//	require.Nil(t, sigBytes)
//}
//
//func TestGetAppchains(t *testing.T) {
//	adapter, client, _ := prepare(t, 1)
//	defer adapter.storage.Close()
//
//	// set up return metaReceipt
//	chainInfo := &appchainmgr.Appchain{
//		Status:    governance.GovernanceAvailable,
//		ID:        from,
//		TrustRoot: nil,
//		Broker:    "",
//		Desc:      "",
//		Version:   0,
//	}
//
//	//	&appchainmgr.Appchain{
//	//	ID:            from,
//	//	Name:          "fabric",
//	//	Validators:    "fabric",
//	//	ConsensusType: "rbft",
//	//	Status:        governance.GovernanceAvailable,
//	//	ChainType:     "fabric",
//	//}
//
//	originalChainsInfo := []*appchainmgr.Appchain{chainInfo, chainInfo}
//	info, err := json.Marshal(originalChainsInfo)
//	require.Nil(t, err)
//
//	chainsReceipt := &pb.Receipt{
//		Ret:    info,
//		Status: 0,
//	}
//	getAppchainsTx := getTx(t)
//
//	// test for normal flow
//	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
//	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
//	chains, err := adapter.GetAppchains()
//	require.Nil(t, err)
//	require.Equal(t, originalChainsInfo, chains)
//
//	// test for abnormal flow: nil return bytes in receipt
//	chainsReceipt.Ret = nil
//	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
//	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
//	chains, err = adapter.GetAppchains()
//	require.Nil(t, err)
//	require.Equal(t, 0, len(chains))
//
//	// test for abnormal flow: error when invoke contract
//	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(nil, fmt.Errorf("test error"))
//	chains, err = adapter.GetAppchains()
//	require.NotNil(t, err)
//	require.Nil(t, nil)
//
//	// test for abnormal flow: invalid receipt ret bytes
//	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
//	client.EXPECT().SendView(getAppchainsTx).Return(nil, fmt.Errorf("test error"))
//	chainsReceipt.Ret = []byte("test for abnormal flow")
//	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
//	chains, err = adapter.GetAppchains()
//	require.NotNil(t, err)
//	require.Nil(t, chains)
//}

func TestSendIBTP(t *testing.T) {
	adapter, client := prepare(t)

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
	client.EXPECT().InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, gomock.Any()).Return(receipt, nil).AnyTimes()

	//err := adapter.RegisterRollbackHandler(func(ibtp *pb.IBTP, ibtpId string) {})
	//require.Nil(t, err)

	// test for receipt failed situation
	client.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).Return(hash, nil).AnyTimes()
	//errMsg1 := fmt.Sprintf("%s", ibtpIndexExist)
	errMsg2 := fmt.Sprintf("%s: appchain not registerd", TargetAppchainNotAvailable)
	//errMsg3 := fmt.Sprintf("%s: ibtp verify error", invalidIBTP)
	errMsg4 := fmt.Sprintf("%s: ibtp already on chain", ibtpIndexExist)
	errMsg5 := fmt.Sprintf("%s: ibtp index is too high", ibtpIndexWrong)
	failReceipt := &pb.Receipt{
		Ret:    []byte(errMsg2),
		Status: pb.Receipt_FAILED,
	}
	client.EXPECT().GetReceipt(hash).Return(failReceipt, nil).AnyTimes()
	require.Nil(t, adapter.SendIBTP(&pb.IBTP{}))

	//failReceipt.Ret = []byte(errMsg2)
	//require.Nil(t, adapter.SendIBTP(&pb.IBTP{}))

	//failReceipt.Ret = []byte(errMsg3)
	//err = adapter.SendIBTP(&pb.IBTP{})
	//require.Nil(t, err)

	// this ibtp sending should be ignored
	failReceipt.Ret = []byte(errMsg4)
	err := adapter.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)

	failReceipt.Ret = []byte(errMsg5)
	err = adapter.SendIBTP(&pb.IBTP{})
	require.NotNil(t, err)
	fmt.Printf("err is %s\n", err.Error())
	require.Equal(t, true, strings.Contains(err.Error(), ibtpIndexWrong))
}

func TestQueryInterchainMeta(t *testing.T) {
	adapter, client := prepare(t)

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
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetInterchain", gomock.Any()).Return(queryTx, nil).AnyTimes()
	client.EXPECT().SendView(queryTx).Return(badReceipt, nil)
	client.EXPECT().SendView(queryTx).Return(badReceipt1, nil)
	client.EXPECT().SendView(queryTx).Return(normalReceipt, nil)

	loggers.InitializeLogger(repo.DefaultConfig())
	meta, err := adapter.QueryInterchain(from)
	require.Nil(t, err)
	require.Equal(t, originalMeta.InterchainCounter[from], meta.InterchainCounter[from])
}

func prepare(t *testing.T) (*BxhAdapter, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	client := mock_client.NewMockClient(mockCtl)

	config := &repo.Config{}
	config.Mode.Type = repo.RelayMode

	adapter, err := New(from, repo.RelayMode,
		client, log.NewWithModule("adapter"),
	)
	require.Nil(t, err)

	subCh := make(chan interface{}, 2)
	client.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(subCh, nil).AnyTimes()
	client.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).AnyTimes()

	return adapter, client
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

func getTx(t *testing.T) *pb.BxhTransaction {
	ibtp := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
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

func getEmptyIBTPTx(t *testing.T) *pb.BxhTransaction {
	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: []byte("empty ibtp payload"),
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From:    faddr,
		To:      faddr,
		IBTP:    nil,
		Payload: data,
	}
	tx.TransactionHash = tx.Hash()
	return tx
}

func getUnoinTx(t *testing.T) *pb.BxhTransaction {
	unionIbtp := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: nil,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.BxhTransaction{
		From:    faddr,
		To:      faddr,
		IBTP:    unionIbtp,
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
