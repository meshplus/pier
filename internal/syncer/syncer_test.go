package syncer

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/meshplus/pier/pkg/model"

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

func TestSyncBlock(t *testing.T) {
	syncer, ag := prepare(t)
	defer syncer.storage.Close()

	// expect mock module returns
	recoverC := make(chan *pb.MerkleWrapper, 1)
	syncC := make(chan *pb.MerkleWrapper, 1)
	txs := make([]*pb.Transaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	w1 := getWrapper(txs, &pb.BlockHeader{
		Number: uint64(0),
	})
	recoverC <- w1

	meta := &pb.ChainMeta{
		Height:    0,
		BlockHash: types.String2Hash(from),
	}
	ag.EXPECT().SyncMerkleWrapper(gomock.Any()).Return(syncC, nil).AnyTimes()
	ag.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	ag.EXPECT().GetMerkleWrapper(gomock.Any(), gomock.Any()).Return(recoverC, nil).AnyTimes()

	done := make(chan bool)
	go func() {
		err := syncer.Start()
		require.Nil(t, err)
		<-done
	}()

	w2 := getWrapper(txs, &pb.BlockHeader{
		Number: uint64(1),
	})
	syncC <- w2
	time.Sleep(1 * time.Second)
	receivedWrapper := &pb.MerkleWrapper{}
	val, err := syncer.storage.Get(model.WrapperKey(1))
	require.Nil(t, err)

	err = receivedWrapper.Unmarshal(val)
	require.Nil(t, err)
	require.Equal(t, w2, receivedWrapper)

	done <- true
	require.Nil(t, syncer.Stop())
}

func prepare(t *testing.T) (*MerkleSyncer, *mock_agent.MockAgent) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	ag := mock_agent.NewMockAgent(mockCtl)
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	validators := make([]types.Address, 0)
	validators = append(validators,
		types.String2Address("0x000f1a7a08ccc48e5d30f80850cf1cf283aa3abd"),
		types.String2Address("0xe93b92f1da08f925bdee44e91e7768380ae83307"),
		types.String2Address("0xb18c8575e3284e79b92100025a31378feb8100d6"),
		types.String2Address("0x856E2B9A5FA82FD1B031D1FF6863864DBAC7995D"),
	)
	syncer, err := New(ag, validators, storage)
	require.Nil(t, err)
	return syncer, ag
}

func getWrapper(txs []*pb.Transaction, h *pb.BlockHeader) *pb.MerkleWrapper {
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
		Signatures:        nil,
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
	pd := &pb.Payload{
		FID:  from,
		TID:  from,
		Func: "set",
		Args: [][]byte{[]byte("Alice")},
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
