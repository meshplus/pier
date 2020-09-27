package syncer

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/meshplus/pier/internal/repo"

	"github.com/cbergoon/merkletree"
	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/internal/lite/mock_lite"
	"github.com/meshplus/pier/pkg/model"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestSyncHeader(t *testing.T) {
	syncer, ag, lite := prepare(t)
	defer syncer.storage.Close()

	// expect mock module returns
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	txs1 := make([]*pb.Transaction, 0, 2)
	txs1 = append(txs1, getTx(t), getTx(t))

	w1, _ := getTxWrapper(txs, txs1, 1)
	w2, root := getTxWrapper(txs, txs1, 2)
	h2 := getBlockHeader(root, 2)
	// mock invalid tx wrapper
	w3, _ := getTxWrapper(txs, txs1, 3)
	w3.InterchainTxWrappers[0].TransactionHashes = w3.InterchainTxWrappers[0].TransactionHashes[1:]

	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncInterchainTxWrappers(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, ch chan *pb.InterchainTxWrappers) {
		ch <- w2
		ch <- w3
	}).AnyTimes()
	ag.EXPECT().GetInterchainTxWrappers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, begin, end uint64, ch chan *pb.InterchainTxWrappers) {
			ch <- w1
			close(ch)
		}).AnyTimes()
	ag.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	lite.EXPECT().QueryHeader(gomock.Any()).Return(h2, nil).AnyTimes()

	done := make(chan bool, 1)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
	}()

	time.Sleep(1 * time.Second)

	// recover should have persist height 1 wrapper
	receiveWrapper := &pb.InterchainTxWrappers{}
	val, err := syncer.storage.Get(model.WrapperKey(2))
	require.Nil(t, err)

	require.Nil(t, receiveWrapper.Unmarshal(val))
	require.Equal(t, w2, receiveWrapper)
	done <- true
	require.Equal(t, uint64(2), syncer.height)
	require.Nil(t, syncer.Stop())
}

func prepare(t *testing.T) (*WrapperSyncer, *mock_agent.MockAgent, *mock_lite.MockLite) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	ag := mock_agent.NewMockAgent(mockCtl)
	lite := mock_lite.NewMockLite(mockCtl)

	config := &repo.Config{}
	config.Mode.Type = repo.DirectMode

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	syncer, err := New(ag, lite, storage, config)
	require.Nil(t, err)

	// register handler for syncer
	require.Nil(t, syncer.RegisterIBTPHandler(func(ibtp *pb.IBTP) {}))
	require.Nil(t, syncer.RegisterAppchainHandler(func() error { return nil }))
	return syncer, ag, lite
}

func getBlockHeader(root types.Hash, number uint64) *pb.BlockHeader {
	wrapper := &pb.BlockHeader{
		Number:     number,
		Timestamp:  time.Now().UnixNano(),
		ParentHash: types.String2Hash(from),
		TxRoot:     root,
	}

	return wrapper
}

func getTxWrapper(interchainTxs []*pb.Transaction, innerchainTxs []*pb.Transaction, number uint64) (*pb.InterchainTxWrappers, types.Hash) {
	var l2roots []types.Hash
	var interchainTxHashes []types.Hash
	hashes := make([]merkletree.Content, 0, len(interchainTxs))
	for i := 0; i < len(interchainTxs); i++ {
		hashes = append(hashes, pb.TransactionHash(interchainTxs[i].Hash().Bytes()))
		interchainTxHashes = append(interchainTxHashes, interchainTxs[i].Hash())
	}
	tree, _ := merkletree.NewTree(hashes)
	l2roots = append(l2roots, types.Bytes2Hash(tree.MerkleRoot()))

	hashes = make([]merkletree.Content, 0, len(innerchainTxs))
	for i := 0; i < len(innerchainTxs); i++ {
		hashes = append(hashes, pb.TransactionHash(innerchainTxs[i].Hash().Bytes()))
	}
	tree, _ = merkletree.NewTree(hashes)
	l2roots = append(l2roots, types.Bytes2Hash(tree.MerkleRoot()))

	contents := make([]merkletree.Content, 0, len(l2roots))
	for _, root := range l2roots {
		contents = append(contents, pb.TransactionHash(root.Bytes()))
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
	return itw, types.Bytes2Hash(l1root)
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

	data := &pb.TransactionData{
		Type:    pb.TransactionData_INVOKE,
		Payload: pd,
	}

	faddr := types.Address{}
	faddr.SetBytes([]byte(from))
	tx := &pb.Transaction{
		From: faddr,
		To:   faddr,
		Data: data,
	}
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
