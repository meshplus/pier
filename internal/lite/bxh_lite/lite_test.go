package bxh_lite

import (
	"context"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/merkle/merkletree"
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
	lite, ag, _ := prepare(t)
	defer lite.storage.Close()

	// expect mock module returns
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	h1 := getBlockHeader(t, txs, 1)
	h2 := getBlockHeader(t, txs, 2)
	// mock invalid block header
	h3 := getBlockHeader(t, txs, 3)
	h3.TxRoot = types.String2Hash(from)

	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncBlockHeader(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, ch chan *pb.BlockHeader) {
		ch <- h2
		close(ch)
	}).AnyTimes()
	ag.EXPECT().GetBlockHeader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, begin, end uint64, ch chan *pb.BlockHeader) {
			ch <- h1
			close(ch)
		}).AnyTimes()
	ag.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()

	done := make(chan bool, 1)
	go func() {
		err := lite.Start()
		require.Nil(t, err)
		<-done
	}()

	time.Sleep(1 * time.Second)

	// recover should have persist height 1 wrapper
	receivedHeader, err := lite.QueryHeader(2)
	require.Nil(t, err)

	require.Equal(t, h2, receivedHeader)
	done <- true
	require.Equal(t, uint64(2), lite.height)
	require.Nil(t, lite.Stop())
}

func prepare(t *testing.T) (*BxhLite, *mock_agent.MockAgent, []crypto.PrivateKey) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	ag := mock_agent.NewMockAgent(mockCtl)
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	keys := getVlts(t)
	var vltSet []types.Address
	for _, key := range keys {
		vlt, err := key.PublicKey().Address()
		require.Nil(t, err)
		vltSet = append(vltSet, vlt)
	}

	lite, err := New(ag, storage)
	require.Nil(t, err)

	return lite, ag, keys
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

func getVlts(t *testing.T) []crypto.PrivateKey {
	var keys []crypto.PrivateKey
	for i := 0; i < 4; i++ {
		priv, err := ecdsa.GenerateKey(ecdsa.Secp256r1)
		require.Nil(t, err)
		keys = append(keys, priv)
	}
	return keys
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
	pd := &pb.Payload{
		SrcContractId: from,
		DstContractId: from,
		Func:          "set",
		Args:          [][]byte{[]byte("Alice")},
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
