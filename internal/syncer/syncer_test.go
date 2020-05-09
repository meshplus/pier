package syncer

import (
	"context"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/meshplus/bitxhub-kit/merkle/merkletree"

	"github.com/meshplus/pier/pkg/model"

	"github.com/meshplus/pier/internal/lite/mock_lite"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent/mock_agent"
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

	w1 := getTxWrapper(t, txs, 1)
	h2 := getBlockHeader(t, txs, 2)
	w2 := getTxWrapper(t, txs, 2)
	// mock invalid tx wrapper
	w3 := getTxWrapper(t, txs, 3)
	w3.TransactionHashes = w3.TransactionHashes[1:]

	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncInterchainTxWrapper(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, ch chan *pb.InterchainTxWrapper) {
		ch <- w2
		ch <- w3
	}).AnyTimes()
	ag.EXPECT().GetInterchainTxWrapper(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, begin, end uint64, ch chan *pb.InterchainTxWrapper) {
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
	receiveWrapper := &pb.InterchainTxWrapper{}
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

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	syncer, err := New(ag, lite, storage)
	require.Nil(t, err)

	return syncer, ag, lite
}

func getBlockHeader(t *testing.T, txs []*pb.Transaction, number uint64) *pb.BlockHeader {
	hashes := make([]interface{}, 0, len(txs))
	for i := 0; i < len(txs); i++ {
		hash := txs[i].Hash()
		hashes = append(hashes, pb.TransactionHash(hash.Bytes()))
	}

	tree := merkletree.NewMerkleTree()
	require.Nil(t, tree.InitMerkleTree(hashes))
	root := tree.GetMerkleRoot()

	wrapper := &pb.BlockHeader{
		Number:     number,
		Timestamp:  time.Now().UnixNano(),
		ParentHash: types.String2Hash(from),
		TxRoot:     types.Bytes2Hash(root),
	}

	return wrapper
}

func getTxWrapper(t *testing.T, txs []*pb.Transaction, number uint64) *pb.InterchainTxWrapper {
	hashes := make([]types.Hash, 0, len(txs))
	for i := 0; i < len(txs); i++ {
		hash := txs[i].Hash()
		hashes = append(hashes, hash)
	}

	wrapper := &pb.InterchainTxWrapper{
		Transactions:      txs,
		TransactionHashes: hashes,
		Height:            number,
	}

	return wrapper
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
		From:  faddr,
		To:    faddr,
		Data:  data,
		Nonce: rand.Int63(),
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
