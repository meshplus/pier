package syncer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/meshplus/bitxhub-kit/log"

	"github.com/cbergoon/merkletree"
	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/meshplus/pier/internal/lite/mock_lite"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/pkg/model"
	"github.com/stretchr/testify/require"
)

const (
	hash = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

// 1. test normal interchain wrapper
// 2. test interchain wrappers arriving at wrong sequence
// 3. test invalid interchain wrappers
func TestSyncHeader001(t *testing.T) {
	syncer, client, lite := prepare(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	syncer.ctx = ctx
	syncer.cancel = cancel
	defer syncer.storage.Close()

	// expect mock module returns
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	txs1 := make([]*pb.Transaction, 0, 2)
	txs1 = append(txs1, getTx(t), getTx(t))

	w1, _ := getTxWrapper(t, txs, txs1, 1)
	w2, _ := getTxWrapper(t, txs, txs1, 2)
	w3, root := getTxWrapper(t, txs, txs1, 3)
	h3 := getBlockHeader(root, 3)
	// mock invalid tx wrapper
	w4, _ := getTxWrapper(t, txs, txs1, 4)
	w4.InterchainTxWrappers[0].TransactionHashes = w4.InterchainTxWrappers[0].TransactionHashes[1:]
	// mock nil wrapper
	w5 := &pb.InterchainTxWrappers{}
	// mock wrong height wrapper
	w6, _ := getTxWrapper(t, txs, txs1, 1)
	w7 := &pb.InterchainTxWrappers{InterchainTxWrappers: []*pb.InterchainTxWrapper{nil}}
	// mock bad tx wrapper
	badTxs := make([]*pb.Transaction, 0, 2)
	badTxs = append(badTxs, getUnoinTx(t), getUnoinTx(t))

	badTxs2 := make([]*pb.Transaction, 0, 2)
	badTxs2 = append(badTxs2, getUnoinTx(t), getUnoinTx(t))
	w8, _ := getTxWrapper(t, badTxs, badTxs2, 4)

	syncWrapperCh := make(chan interface{}, 4)
	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.NewHashByStr(from),
	}
	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER, gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
	client.EXPECT().GetInterchainTxWrappers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, pid string, begin, end uint64, ch chan<- *pb.InterchainTxWrappers) {
			if end == 1 {
				ch <- w1
			} else {
				// for wrong path handling
				ch <- w5
				ch <- w6
				ch <- w7
				// for normal handling
				ch <- w2
				ch <- w3
			}
			close(ch)
		}).AnyTimes()
	client.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	queryCall1 := lite.EXPECT().QueryHeader(gomock.Any()).Return(nil, fmt.Errorf("wrong query header"))
	queryCall2 := lite.EXPECT().QueryHeader(gomock.Any()).Return(h3, nil).MaxTimes(10)
	gomock.InOrder(queryCall1, queryCall2)
	go func() {
		syncWrapperCh <- w3
		syncWrapperCh <- w4
		syncWrapperCh <- w5
		syncWrapperCh <- w6
		syncWrapperCh <- w7
		syncWrapperCh <- w8
	}()

	syncer.height = 1
	ok, err := syncer.verifyWrapper(w2.InterchainTxWrappers[0])
	require.Nil(t, err)
	require.Equal(t, true, ok)

	done := make(chan bool, 1)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
		require.Nil(t, syncer.Stop())
	}()

	time.Sleep(1 * time.Second)

	// recover should have persist height 3 wrapper
	receiveWrapper := &pb.InterchainTxWrappers{}
	val := syncer.storage.Get(model.WrapperKey(3))

	require.Nil(t, receiveWrapper.Unmarshal(val))
	for i, tx := range w3.InterchainTxWrappers[0].Transactions {
		require.Equal(t, tx.TransactionHash.String(), receiveWrapper.InterchainTxWrappers[0].Transactions[i].TransactionHash.String())
	}
	done <- true
	require.Equal(t, uint64(3), syncer.height)
}

func TestSyncUnoinHeader(t *testing.T) {
	syncer, _, lite := prepare(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	syncer.ctx = ctx
	syncer.cancel = cancel
	defer syncer.storage.Close()

	// expect mock module returns
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getUnoinTx(t), getUnoinTx(t))

	txs1 := make([]*pb.Transaction, 0, 2)
	txs1 = append(txs1, getUnoinTx(t), getUnoinTx(t))

	w1, _ := getTxWrapper(t, txs, txs1, 1)
	w2, root := getTxWrapper(t, txs, txs1, 2)
	h2 := getBlockHeader(root, 2)

	lite.EXPECT().QueryHeader(gomock.Any()).Return(h2, nil).AnyTimes()

	syncer.isRecover = true
	syncer.mode = repo.UnionMode
	icm := make(map[string]*rpcx.Interchain)
	// recover for error and normal situation
	syncer.RegisterRecoverHandler(recoverFail)
	syncer.handleInterchainWrapperAndPersist(w1, icm)
	syncer.RegisterRecoverHandler(recoverHandler)
	syncer.handleInterchainWrapperAndPersist(w1, icm)
	syncer.height = 1
	syncer.handleInterchainWrapperAndPersist(w2, icm)

	time.Sleep(1 * time.Second)
	require.Equal(t, uint64(2), syncer.height)
}

func recoverFail(_ *pb.IBTP) (*rpcx.Interchain, error) {
	return nil, fmt.Errorf("recover interchain meta failed")
}

func recoverHandler(ibtp *pb.IBTP) (*rpcx.Interchain, error) {
	return &rpcx.Interchain{
		ID:                   from,
		InterchainCounter:    map[string]uint64{ibtp.To: 1},
		SourceReceiptCounter: map[string]uint64{},
		ReceiptCounter:       map[string]uint64{},
	}, nil
}

func TestSyncHeader002(t *testing.T) {
	syncer, client, lite := prepare(t, 1)
	defer syncer.storage.Close()

	// expect mock module returns
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	txs1 := make([]*pb.Transaction, 0, 2)
	txs1 = append(txs1, getTx(t), getTx(t))

	w2, _ := getTxWrapper(t, txs, txs1, 2)
	w3, root := getTxWrapper(t, txs, txs1, 3)
	h3 := getBlockHeader(root, 3)
	// mock invalid tx wrapper
	w4, _ := getTxWrapper(t, txs, txs1, 4)
	w4.InterchainTxWrappers[0].TransactionHashes = w4.InterchainTxWrappers[0].TransactionHashes[1:]

	syncWrapperCh := make(chan interface{}, 4)
	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.NewHashByStr(from),
	}
	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER, gomock.Any()).Return(syncWrapperCh, nil).AnyTimes()
	client.EXPECT().GetInterchainTxWrappers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, pid string, begin, end uint64, ch chan<- *pb.InterchainTxWrappers) {
			if begin == 2 {
				ch <- w2
			} else if begin == 3 {
				ch <- w3
			}
			close(ch)
		}).AnyTimes()
	client.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	lite.EXPECT().QueryHeader(gomock.Any()).Return(h3, nil).AnyTimes()
	go func() {
		syncWrapperCh <- w3
		syncWrapperCh <- w4
	}()

	syncer.height = 1
	ok, err := syncer.verifyWrapper(w2.InterchainTxWrappers[0])
	require.Nil(t, err)
	require.Equal(t, true, ok)

	done := make(chan bool, 1)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
	}()

	time.Sleep(1 * time.Second)

	// recover should have persist height 3 wrapper
	receiveWrapper := &pb.InterchainTxWrappers{}
	val := syncer.storage.Get(model.WrapperKey(3))

	require.Nil(t, receiveWrapper.Unmarshal(val))
	for i, tx := range w3.InterchainTxWrappers[0].Transactions {
		require.Equal(t, tx.TransactionHash.String(), receiveWrapper.InterchainTxWrappers[0].Transactions[i].TransactionHash.String())
	}
	done <- true
	require.Equal(t, uint64(3), syncer.height)
	require.Nil(t, syncer.Stop())
}

// 4. test broken network when sending ibtp
func TestQueryIBTP(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	origin := &pb.IBTP{
		From:      from,
		Index:     1,
		Timestamp: time.Now().UnixNano(),
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

	tx := &pb.Transaction{
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
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(tx, nil).AnyTimes()
	// test for normal receipt
	client.EXPECT().SendView(tx).Return(normalReceipt, nil)
	ibtp, err := syncer.QueryIBTP(from)
	require.Nil(t, err)
	require.Equal(t, origin, ibtp)

	// test for abnormal receipt
	client.EXPECT().SendView(tx).Return(badReceipt, nil)
	ibtp, err = syncer.QueryIBTP(from)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	client.EXPECT().SendView(tx).Return(badReceipt1, nil)
	ibtp, err = syncer.QueryIBTP(from)
	require.NotNil(t, err)
	require.Nil(t, ibtp)
}

func TestGetAssetExchangeSigns(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	assetExchangeID := fmt.Sprintf("%s", from)
	digest := sha256.Sum256([]byte(assetExchangeID))
	priv, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	sig, err := priv.Sign(digest[:])
	require.Nil(t, err)
	resp := &pb.SignResponse{
		Sign: map[string][]byte{assetExchangeID: sig},
	}

	// test for normal response
	client.EXPECT().GetMultiSigns(assetExchangeID, pb.GetMultiSignsRequest_ASSET_EXCHANGE).
		Return(resp, nil)
	sigBytes, err := syncer.GetAssetExchangeSigns(assetExchangeID)
	require.Nil(t, err)
	require.Equal(t, true, bytes.Equal(sigBytes, sigBytes))

	// test for abnormal receipt
	client.EXPECT().GetMultiSigns(assetExchangeID, pb.GetMultiSignsRequest_ASSET_EXCHANGE).
		Return(nil, nil)
	sigBytes, err = syncer.GetAssetExchangeSigns(assetExchangeID)
	require.NotNil(t, err)
	require.Nil(t, sigBytes)

	resp.Sign = nil
	client.EXPECT().GetMultiSigns(assetExchangeID, pb.GetMultiSignsRequest_ASSET_EXCHANGE).
		Return(resp, nil)
	sigBytes, err = syncer.GetAssetExchangeSigns(assetExchangeID)
	require.NotNil(t, err)
	require.Nil(t, sigBytes)
}

func TestGetIBTPSigns(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	ibtp := &pb.IBTP{}
	hash := ibtp.Hash().String()

	digest := sha256.Sum256([]byte(hash))
	priv, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	sig, err := priv.Sign(digest[:])
	require.Nil(t, err)
	resp := &pb.SignResponse{
		Sign: map[string][]byte{hash: sig},
	}

	// test for normal response
	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
		Return(resp, nil)
	sigBytes, err := syncer.GetIBTPSigns(ibtp)
	require.Nil(t, err)
	require.Equal(t, true, bytes.Equal(sigBytes, sigBytes))

	// test for abnormal receipt
	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
		Return(nil, nil)
	sigBytes, err = syncer.GetIBTPSigns(ibtp)
	require.NotNil(t, err)
	require.Nil(t, sigBytes)

	resp.Sign = nil
	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
		Return(resp, nil)
	sigBytes, err = syncer.GetIBTPSigns(ibtp)
	require.NotNil(t, err)
	require.Nil(t, sigBytes)

	client.EXPECT().GetMultiSigns(hash, pb.GetMultiSignsRequest_IBTP).
		Return(nil, fmt.Errorf("test error"))
	sigBytes, err = syncer.GetIBTPSigns(ibtp)
	require.NotNil(t, err)
	require.Nil(t, sigBytes)
}

func TestGetAppchains(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	// set up return metaReceipt
	chainInfo := &rpcx.Appchain{
		ID:            from,
		Name:          "fabric",
		Validators:    "fabric",
		ConsensusType: 0,
		Status:        0,
		ChainType:     "fabric",
	}
	originalChainsInfo := []*rpcx.Appchain{chainInfo, chainInfo}
	info, err := json.Marshal(originalChainsInfo)
	require.Nil(t, err)

	chainsReceipt := &pb.Receipt{
		Ret:    info,
		Status: 0,
	}
	getAppchainsTx := getTx(t)

	// test for normal flow
	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
	chains, err := syncer.GetAppchains()
	require.Nil(t, err)
	require.Equal(t, originalChainsInfo, chains)

	// test for abnormal flow: nil return bytes in receipt
	chainsReceipt.Ret = nil
	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
	chains, err = syncer.GetAppchains()
	require.Nil(t, err)
	require.Equal(t, 0, len(chains))

	// test for abnormal flow: error when invoke contract
	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(nil, fmt.Errorf("test error"))
	chains, err = syncer.GetAppchains()
	require.NotNil(t, err)
	require.Nil(t, nil)

	// test for abnormal flow: invalid receipt ret bytes
	client.EXPECT().GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), gomock.Any()).Return(getAppchainsTx, nil)
	client.EXPECT().SendView(getAppchainsTx).Return(nil, fmt.Errorf("test error"))
	chainsReceipt.Ret = []byte("test for abnormal flow")
	client.EXPECT().SendView(getAppchainsTx).Return(chainsReceipt, nil)
	chains, err = syncer.GetAppchains()
	require.NotNil(t, err)
	require.Nil(t, chains)
}

func TestSendIBTP(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	b := &types.Address{}
	b.SetBytes([]byte(from))
	tx := &pb.Transaction{
		From: b,
	}

	r := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: 0,
	}
	client.EXPECT().GenerateIBTPTx(gomock.Any()).Return(tx, nil).AnyTimes()
	origin := &pb.IBTP{
		From:      from,
		Index:     1,
		Timestamp: time.Now().UnixNano(),
	}
	receiptData, err := origin.Marshal()
	require.Nil(t, err)
	ibtpReceipt := &pb.Receipt{
		Ret:    receiptData,
		Status: pb.Receipt_SUCCESS,
	}
	queryIBTPTx := &pb.Transaction{}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetIBTPByID", gomock.Any()).Return(queryIBTPTx, nil).AnyTimes()
	client.EXPECT().SendView(queryIBTPTx).Return(ibtpReceipt, nil)
	networkDownTime := 0
	client.EXPECT().SendTransactionWithReceipt(gomock.Any(), gomock.Any()).DoAndReturn(
		func(tx *pb.Transaction, opts *rpcx.TransactOpts) (*pb.Receipt, error) {
			networkDownTime++
			if networkDownTime == 1 {
				r.Status = pb.Receipt_FAILED
				return r, nil
			} else if networkDownTime == 2 {
				return nil, fmt.Errorf("network broken")
			}
			r.Status = pb.Receipt_SUCCESS
			return r, nil
		}).AnyTimes()
	require.Nil(t, syncer.SendIBTP(&pb.IBTP{}))
}

func TestGetInterchainById(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()

	b := &types.Address{}
	b.SetBytes([]byte(from))
	tx := &pb.Transaction{
		From: b,
	}
	ic := &pb.Interchain{
		ID:                from,
		InterchainCounter: make(map[string]uint64),
	}
	receiptData, err := ic.Marshal()
	require.Nil(t, err)
	normalReceipt := &pb.Receipt{
		Ret:    receiptData,
		Status: pb.Receipt_SUCCESS,
	}
	badReceipt := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: pb.Receipt_SUCCESS,
	}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "GetInterchain", rpcx.String(from)).Return(tx, nil).AnyTimes()

	// test for normal
	client.EXPECT().SendView(tx).Return(normalReceipt, nil)
	retIC := syncer.GetInterchainById(from)
	require.Equal(t, ic.ID, retIC.ID)

	// test for wrong receipt
	client.EXPECT().SendView(tx).Return(badReceipt, nil)
	retIC = syncer.GetInterchainById(from)
	require.Equal(t, "", retIC.ID)
}

func TestQueryInterchainMeta(t *testing.T) {
	syncer, client, _ := prepare(t, 1)
	defer syncer.storage.Close()
	b := &types.Address{}
	b.SetBytes([]byte(from))
	queryTx := &pb.Transaction{
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
		Status: pb.Receipt_SUCCESS,
	}
	badReceipt1 := &pb.Receipt{
		Ret:    []byte("this is a failed receipt"),
		Status: pb.Receipt_FAILED,
	}
	client.EXPECT().GenerateContractTx(gomock.Any(), gomock.Any(), "Interchain").Return(queryTx, nil).AnyTimes()
	client.EXPECT().SendView(queryTx).Return(badReceipt, nil)
	client.EXPECT().SendView(queryTx).Return(badReceipt1, nil)
	client.EXPECT().SendView(queryTx).Return(normalReceipt, nil)

	meta := syncer.QueryInterchainMeta()
	require.Equal(t, originalMeta.InterchainCounter[from], meta[from])
}

func prepare(t *testing.T, height uint64) (*WrapperSyncer, *mock_client.MockClient, *mock_lite.MockLite) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	client := mock_client.NewMockClient(mockCtl)
	lite := mock_lite.NewMockLite(mockCtl)

	config := &repo.Config{}
	config.Mode.Type = repo.RelayMode

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	syncer, err := New(from, repo.RelayMode,
		WithClient(client), WithLite(lite), WithStorage(storage),
		WithLogger(log.NewWithModule("syncer")),
	)
	require.Nil(t, err)

	if height != 0 {
		syncer.storage.Put(syncHeightKey(), []byte(strconv.FormatUint(height, 10)))
	}
	// register handler for syncer
	require.Nil(t, syncer.RegisterAppchainHandler(func() error { return nil }))
	return syncer, client, lite
}

func getBlockHeader(root *types.Hash, number uint64) *pb.BlockHeader {
	wrapper := &pb.BlockHeader{
		Number:     number,
		Timestamp:  time.Now().UnixNano(),
		ParentHash: types.NewHashByStr(from),
		TxRoot:     root,
	}

	return wrapper
}

func getTxWrapper(t *testing.T, interchainTxs []*pb.Transaction, innerchainTxs []*pb.Transaction, number uint64) (*pb.InterchainTxWrappers, *types.Hash) {
	var l2roots []types.Hash
	var interchainTxHashes []types.Hash
	hashes := make([]merkletree.Content, 0, len(interchainTxs))
	for i := 0; i < len(interchainTxs); i++ {
		interchainHash := interchainTxs[i].Hash()
		hashes = append(hashes, interchainHash)
		interchainTxHashes = append(interchainTxHashes, *interchainHash)
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
		Transactions:      interchainTxs,
		TransactionHashes: interchainTxHashes,
		Height:            number,
		L2Roots:           l2roots,
	}
	wrappers = append(wrappers, wrapper)
	itw := &pb.InterchainTxWrappers{
		InterchainTxWrappers: wrappers,
	}
	return itw, types.NewHash(l1root)
}

func getTx(t *testing.T) *pb.Transaction {
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
	tx := &pb.Transaction{
		From:    faddr,
		To:      faddr,
		IBTP:    ibtp,
		Payload: data,
	}
	tx.TransactionHash = tx.Hash()
	return tx
}

func getEmptyIBTPTx(t *testing.T) *pb.Transaction {
	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: []byte("empty ibtp payload"),
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.Transaction{
		From:    faddr,
		To:      faddr,
		IBTP:    nil,
		Payload: data,
	}
	tx.TransactionHash = tx.Hash()
	return tx
}

func getUnoinTx(t *testing.T) *pb.Transaction {
	unionIbtp := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	td := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: nil,
	}
	data, err := td.Marshal()
	require.Nil(t, err)

	faddr := &types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.Transaction{
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
		SrcContractId: from,
		DstContractId: from,
		Func:          "set",
		Args:          [][]byte{[]byte("Alice")},
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
