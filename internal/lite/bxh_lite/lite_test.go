package bxh_lite

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/cbergoon/merkletree"
	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestSyncHeader(t *testing.T) {
	lite, client, _ := prepare(t)
	defer lite.storage.Close()

	// expect mock module returns
	txs := make([]*pb.BxhTransaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))

	h1 := getBlockHeader(t, txs, 1)
	h2 := getBlockHeader(t, txs, 2)
	// mock invalid block header
	h3 := getBlockHeader(t, txs, 3)
	h3.TxRoot = types.NewHashByStr(from)

	syncHeaderCh := make(chan interface{}, 2)
	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.NewHashByStr(from),
	}
	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(syncHeaderCh, nil).AnyTimes()
	client.EXPECT().GetBlockHeader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, begin, end uint64, ch chan<- *pb.BlockHeader) {
			ch <- h1
			close(ch)
		}).AnyTimes()
	client.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()

	go func() {
		syncHeaderCh <- h2
	}()
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

	// query nonexistent header
	_, err = lite.QueryHeader(10)
	require.NotNil(t, err)
	// query unmarshal error header
	lite.storage.Put(headerKey(20), []byte("a"))
	_, err = lite.QueryHeader(20)
	require.NotNil(t, err)

	// start with get last height error
	lite.storage.Put(headerHeightKey(), []byte("a"))
	err = lite.Start()
	require.NotNil(t, err)
}

func TestSyncHeader_Start_GetChainMateError(t *testing.T) {
	lite, client, _ := prepare(t)
	defer lite.storage.Close()

	client.EXPECT().GetChainMeta().Return(nil, fmt.Errorf("get chain meta error")).AnyTimes()
	err := lite.Start()
	require.NotNil(t, err)
}

func TestSyncHeader_Recover_GetBlockHeaderError(t *testing.T) {
	lite, client, _ := prepare(t)
	defer lite.storage.Close()

	// expect mock module returns
	txs := make([]*pb.BxhTransaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))
	h1 := getBlockHeader(t, txs, 1)

	syncHeaderCh := make(chan interface{}, 2)
	meta := &pb.ChainMeta{
		Height:    1,
		BlockHash: types.NewHashByStr(from),
	}

	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(syncHeaderCh, nil).AnyTimes()
	client.EXPECT().GetChainMeta().Return(meta, nil).AnyTimes()
	client.EXPECT().GetBlockHeader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(ctx context.Context, begin, end uint64, ch chan<- *pb.BlockHeader) {
			ch <- h1
			close(ch)
		}).Return(fmt.Errorf("get block header error")).AnyTimes()

	err := lite.Start()
	require.Nil(t, err)
}

func TestBxhLite_HandleBlockHeaderError(t *testing.T) {
	lite, _, _ := prepare(t)
	defer lite.storage.Close()

	txs := make([]*pb.BxhTransaction, 0, 2)
	txs = append(txs, getTx(t), getTx(t))
	h1 := getBlockHeader(t, txs, 1)

	lite.height = 2
	// handle nil header
	lite.handleBlockHeader(nil)
	require.Equal(t, uint64(2), lite.height)
	// handle header with number less than demand height
	lite.handleBlockHeader(h1)
	require.Equal(t, uint64(2), lite.height)
}

func TestBxhLite_GetHeaderChannelError(t *testing.T) {
	lite, client, _ := prepare(t)
	defer lite.storage.Close()

	syncHeaderCh := make(chan interface{}, 0)

	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(nil, fmt.Errorf("subscribe error")).Times(1)
	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(syncHeaderCh, nil).AnyTimes()

	go func() {
		lite.getHeaderChannel()
	}()
}

func TestBxhLite_SyncBlockHeaderError(t *testing.T) {
	lite, client, _ := prepare(t)
	defer lite.storage.Close()

	client.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(nil, fmt.Errorf("subscribe error")).AnyTimes()

	ch := make(chan *pb.BlockHeader, maxChSize)
	require.NotNil(t, lite.syncBlockHeader(ch))
}

func prepare(t *testing.T) (*BxhLite, *mock_client.MockClient, []crypto.PrivateKey) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	client := mock_client.NewMockClient(mockCtl)
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	keys := getVlts(t)
	lite, err := New(client, storage, log.NewWithModule("lite"))
	require.Nil(t, err)

	return lite, client, keys
}

func getBlockHeader(t *testing.T, txs []*pb.BxhTransaction, number uint64) *pb.BlockHeader {
	hashes := make([]merkletree.Content, 0, len(txs))
	for i := 0; i < len(txs); i++ {
		hash := txs[i].Hash()
		hashes = append(hashes, hash)
	}

	tree, err := merkletree.NewTree(hashes)
	require.Nil(t, err)
	root := tree.MerkleRoot()

	wrapper := &pb.BlockHeader{
		Number:     number,
		Timestamp:  time.Now().UnixNano(),
		ParentHash: types.NewHashByStr(from),
		TxRoot:     types.NewHash(root),
	}

	return wrapper
}

func getVlts(t *testing.T) []crypto.PrivateKey {
	var keys []crypto.PrivateKey
	for i := 0; i < 4; i++ {
		priv, err := asym.GenerateKeyPair(crypto.Secp256k1)
		require.Nil(t, err)
		keys = append(keys, priv)
	}
	return keys
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
		Payload: data,
		IBTP:    ibtp,
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
