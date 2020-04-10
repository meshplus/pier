package syncer

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/pkg/model"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestSyncBlock(t *testing.T) {
	syncer, ag, privKeys := prepare(t)
	defer syncer.storage.Close()

	// expect mock module returns
	recoverC := make(chan *pb.MerkleWrapper, 1)
	syncC := make(chan *pb.MerkleWrapper, 1)
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	w1 := getWrapper(t, txs, &pb.BlockHeader{
		Number: uint64(1),
	}, privKeys)
	recoverC <- w1

	meta := &pb.ChainMeta{
		Height:    0,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncMerkleWrapper(gomock.Any()).Return(syncC, nil).AnyTimes()
	ag.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	ag.EXPECT().GetMerkleWrapper(gomock.Any(), gomock.Any()).Return(recoverC, nil).AnyTimes()

	done := make(chan bool, 1)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
	}()

	// required height is 1, receiving height 2 wrapper will trigger GetMerkleWrapper
	w2 := getWrapper(t, txs, &pb.BlockHeader{
		Number: uint64(2),
	}, privKeys)
	syncC <- w2
	recoverC <- w2
	time.Sleep(1 * time.Second)
	receivedWrapper := &pb.MerkleWrapper{}
	val, err := syncer.storage.Get(model.WrapperKey(1))
	require.Nil(t, err)

	err = receivedWrapper.Unmarshal(val)
	require.Nil(t, err)
	require.Equal(t, w1, receivedWrapper)
	done <- true
	require.Nil(t, syncer.Stop())
}

func TestRecover(t *testing.T) {
	syncer, ag, privKeys := prepare(t)
	defer syncer.storage.Close()

	// expect mock module returns
	recoverC := make(chan *pb.MerkleWrapper, 2)
	syncC := make(chan *pb.MerkleWrapper, 1)
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	w1 := getWrapper(t, txs, &pb.BlockHeader{
		Number: uint64(1),
	}, privKeys)
	recoverC <- w1

	// mock malicious wrapper
	w2 := getWrapper(t, txs, &pb.BlockHeader{
		Number: uint64(1),
	}, privKeys)
	w2.Signatures = nil
	recoverC <- w2

	meta := &pb.ChainMeta{
		Height:    3,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncMerkleWrapper(gomock.Any()).Return(syncC, nil).AnyTimes()
	ag.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	ag.EXPECT().GetMerkleWrapper(gomock.Any(), gomock.Any()).Return(recoverC, nil).AnyTimes()

	done := make(chan bool, 1)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
	}()

	close(recoverC)
	time.Sleep(1 * time.Second)

	// recover should have persist height 1 wrapper
	receivedWrapper := &pb.MerkleWrapper{}
	val, err := syncer.storage.Get(model.WrapperKey(1))
	require.Nil(t, err)

	err = receivedWrapper.Unmarshal(val)
	require.Nil(t, err)
	require.Equal(t, w1, receivedWrapper)
	done <- true
	require.Nil(t, syncer.Stop())
}

func prepare(t *testing.T) (*MerkleSyncer, *mock_agent.MockAgent, []crypto.PrivateKey) {
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

	syncer, err := New(ag, 2, vltSet, storage)
	require.Nil(t, err)
	return syncer, ag, keys
}

func getWrapper(t *testing.T, txs []*pb.Transaction, h *pb.BlockHeader, privateKeys []crypto.PrivateKey) *pb.MerkleWrapper {
	hashes := make([]types.Hash, 0, len(txs))
	for i := 0; i < len(txs); i++ {
		hash := types.Hash{}
		hash.SetBytes([]byte(from))
		hashes = append(hashes, hash)
	}
	wrapper := &pb.MerkleWrapper{
		BlockHeader:       h,
		TransactionHashes: hashes,
		Transactions:      txs,
	}

	sigs := make(map[string][]byte)
	for _, key := range privateKeys {
		sign, err := key.Sign(wrapper.SignHash().Bytes())
		require.Nil(t, err)
		vlt, err := key.PublicKey().Address()
		require.Nil(t, err)
		sigs[vlt.String()] = sign
	}
	wrapper.Signatures = sigs

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
