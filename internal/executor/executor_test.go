package executor

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/pkg/model"
	"github.com/meshplus/pier/pkg/plugins/client/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestExecute(t *testing.T) {
	exec, ag, cli := prepare(t)
	defer exec.storage.Close()

	// set expect values
	ret := &model.PluginResponse{
		Status: true,
		Result: getIBTP(t, 1, pb.IBTP_INTERCHAIN),
	}
	ag.EXPECT().SendIBTP(gomock.Any()).Return(getReceipt(), nil).AnyTimes()
	ag.EXPECT().GetIBTPByID(gomock.Any()).Return(getIBTP(t, 2, pb.IBTP_INTERCHAIN), nil).Times(1)
	cli.EXPECT().SubmitIBTP(gomock.Any()).Return(ret, nil).AnyTimes()

	txs := make([]*pb.Transaction, 0)

	//receipts := make([]*pb.Receipt, 0)
	typs := []pb.IBTP_Type{pb.IBTP_INTERCHAIN, pb.IBTP_RECEIPT}
	for i := 0; i < 2; i++ {
		tx := getTx(t, uint64(i+1), typs[i])
		tx.TransactionHash = tx.Hash()
		txs = append(txs, tx)
	}

	// add replay tx for triggering ignore tx
	ignoreTx := getTx(t, uint64(2), pb.IBTP_INTERCHAIN)
	ignoreTx.TransactionHash = ignoreTx.Hash()
	txs = append(txs, ignoreTx)

	header1 := &pb.BlockHeader{
		Number: uint64(2),
	}

	// set exec height to 2
	exec.height = 2
	wrap1 := getWrapper(txs, header1)

	exec.applyMerkleWrapper(wrap1)
	time.Sleep(1 * time.Second)
	require.Equal(t, uint64(3), exec.height)

	// execute anther wrong index wrapper and exec height remains the same
	wrap2 := getWrapper(txs, &pb.BlockHeader{
		Number: uint64(4),
	})

	exec.applyMerkleWrapper(wrap2)
	time.Sleep(1 * time.Second)
	require.Equal(t, uint64(4), exec.height)
}

func TestRecovery(t *testing.T) {
	exec, ag, cli := prepare(t)

	m1 := make(map[string]uint64)
	m1[from] = 2
	exec.executeMeta = m1
	m2 := make(map[string]uint64)
	m2[from] = 1
	exec.sourceReceiptMeta = m2

	// set up fake returns for missing receipt
	ret := [][]byte{[]byte("Alice"), []byte("100")}
	ibtp := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	w := make(chan *pb.MerkleWrapper)
	ag.EXPECT().SyncMerkleWrapper(gomock.Any()).Return(w, nil).AnyTimes()
	ag.EXPECT().SendIBTP(gomock.Any()).Return(getReceipt(), nil).AnyTimes()
	ag.EXPECT().GetIBTPByID(gomock.Any()).Return(ibtp, nil).AnyTimes()
	cli.EXPECT().GetInMessage(gomock.Any(), gomock.Any()).Return(ret, nil).AnyTimes()
	require.Nil(t, exec.Start())
	require.Equal(t, uint64(2), exec.sourceReceiptMeta[from])
}

func prepare(t *testing.T) (*ChannelExecutor, *mock_agent.MockAgent, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	ag := mock_agent.NewMockAgent(mockCtl)
	cli := mock_client.NewMockClient(mockCtl)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)
	meta := &rpcx.Appchain{
		ID:            from,
		Name:          "testchain",
		ConsensusType: 0,
		Status:        0,
		ChainType:     "hyperchain",
	}

	cli.EXPECT().GetInMeta().Return(make(map[string]uint64), nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(make(map[string]uint64), nil).AnyTimes()
	exec, err := NewChannelExecutor(ag, cli, meta, storage)
	require.Nil(t, err)
	return exec, ag, cli
}

func getWrapper(txs []*pb.Transaction, h *pb.BlockHeader) *pb.MerkleWrapper {
	hashes := make([]types.Hash, 0, len(txs))
	for _, tx := range txs {
		hashes = append(hashes, tx.TransactionHash)
	}
	wrapper := &pb.MerkleWrapper{
		BlockHeader:       h,
		TransactionHashes: hashes,
		Transactions:      txs,
		Signatures:        nil,
	}
	return wrapper
}

func getReceipt() *pb.Receipt {
	return &pb.Receipt{
		Version: []byte("0.4.5"),
		TxHash:  types.Hash{},
		Ret:     nil,
		Status:  0,
	}
}

func getTx(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.Transaction {
	origin := getIBTP(t, index, typ)
	ib, err := origin.Marshal()
	require.Nil(t, err)

	tmpIP := &pb.InvokePayload{
		Method: "set",
		Args:   []*pb.Arg{{Value: ib}},
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
