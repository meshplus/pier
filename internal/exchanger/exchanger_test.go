package exchanger

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/api"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/executor/mock_executor"
	"github.com/meshplus/pier/internal/monitor/mock_monitor"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	peerMsg "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/syncer/mock_syncer"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to   = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestStartRelay(t *testing.T) {
	mode := "relay"
	mockMonitor, mockExecutor, mockSyncer, mockAgent, mockChecker, store := prepareRelay(t)
	meta := &rpcx.Interchain{}

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithAgent(mockAgent),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)

	// mock one missing ibtp and one normal listened ibtp
	missedOutIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	normalOutIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	missedInIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	receipt := getIBTP(t, 1, pb.IBTP_RECEIPT)
	bxhReceipt := &pb.Receipt{
		Status: pb.Receipt_SUCCESS,
	}
	outCh := make(chan *pb.IBTP, 1)
	outCh <- normalOutIBTP

	outMeta := map[string]uint64{to: 1}
	inMeta := map[string]uint64{to: 1}
	mockMonitor.EXPECT().ListenOnIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryLatestMeta().Return(outMeta)
	mockMonitor.EXPECT().QueryIBTP(missedOutIBTP.ID()).Return(missedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().QueryLatestMeta().Return(inMeta)
	mockExecutor.EXPECT().QueryReceipt(to, uint64(1), gomock.Any()).Return(receipt, nil).AnyTimes()
	mockExecutor.EXPECT().HandleIBTP(gomock.Any()).Return(receipt).AnyTimes()
	mockSyncer.EXPECT().RegisterIBTPHandler(gomock.Any()).Return(nil)
	mockAgent.EXPECT().GetIBTPByID(receipt.ID()).Return(missedInIBTP, nil)
	mockAgent.EXPECT().SendIBTP(gomock.Any()).Return(bxhReceipt, nil).AnyTimes()

	require.Nil(t, mockExchanger.Start())

	// mock one illegal replayed ibtp and one normal ibtp from relay chain
	replayedInIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	normalInIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	mockExchanger.handleIBTP(replayedInIBTP)
	mockExchanger.handleIBTP(normalInIBTP)

	time.Sleep(time.Second)
	require.Nil(t, mockExchanger.Stop())
}

func TestStartDirect(t *testing.T) {
	mode := "direct"
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t)
	meta := &rpcx.Interchain{}

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithChecker(mockChecker), WithPeerMgr(mockPeerMgr),
		WithAPIServer(apiServer), WithStorage(store),
	)
	require.Nil(t, err)

	missedOutIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	normalOutIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	//missedInIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	receipt := getIBTP(t, 1, pb.IBTP_RECEIPT)
	//bxhReceipt := &pb.Receipt{
	//	Status: pb.Receipt_SUCCESS,
	//}
	outCh := make(chan *pb.IBTP, 1)

	receiptBytes, err := receipt.Marshal()
	require.Nil(t, err)
	indices := &struct {
		InterchainIndex uint64 `json:"interchain_index"`
		ReceiptIndex    uint64 `json:"receipt_index"`
	}{}
	metaBytes, err := json.Marshal(indices)
	require.Nil(t, err)

	metaMsg := peermgr.Message(peerMsg.Message_INTERCHAIN_META_GET, true, []byte(from))
	retMetaMsg := peermgr.Message(peerMsg.Message_INTERCHAIN_META_GET, true, metaBytes)
	retMsg := peermgr.Message(peerMsg.Message_ACK, true, receiptBytes)

	outMeta := map[string]uint64{to: 1}
	inMeta := map[string]uint64{to: 1}
	mockMonitor.EXPECT().ListenOnIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryLatestMeta().Return(outMeta)
	mockMonitor.EXPECT().QueryIBTP(missedOutIBTP.ID()).Return(missedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().HandleIBTP(gomock.Any()).Return(receipt).AnyTimes()
	mockExecutor.EXPECT().QueryLatestMeta().Return(inMeta)
	mockExecutor.EXPECT().QueryReceipt(to, uint64(1), gomock.Any()).Return(receipt, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), metaMsg).Return(retMetaMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(retMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().SendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	require.Nil(t, mockExchanger.Start())

	mockExchanger.handleNewConnection(to)

	outCh <- normalOutIBTP

	var stream network.Stream
	ibtpBytes, err := normalOutIBTP.Marshal()
	require.Nil(t, err)
	ibtpMsg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, ibtpBytes)
	mockExchanger.handleSendIBTPMessage(stream, ibtpMsg)

	ibtpGetMsg := peermgr.Message(peerMsg.Message_IBTP_GET, true, []byte(missedOutIBTP.ID()))
	mockExchanger.handleGetIBTPMessage(stream, ibtpGetMsg)

	time.Sleep(2 * time.Second)
	require.Nil(t, mockExchanger.Stop())
}

func prepareRelay(t *testing.T) (
	*mock_monitor.MockMonitor, *mock_executor.MockExecutor,
	*mock_syncer.MockSyncer, *mock_agent.MockAgent,
	*checker.MockChecker, storage.Storage,
) {
	mockCtl := gomock.NewController(t)
	mockMonitor := mock_monitor.NewMockMonitor(mockCtl)
	mockExecutor := mock_executor.NewMockExecutor(mockCtl)
	mockSyncer := mock_syncer.NewMockSyncer(mockCtl)
	mockAgent := mock_agent.NewMockAgent(mockCtl)
	mockChecker := &checker.MockChecker{}

	mockSyncer.EXPECT().Start().Return(nil)
	mockSyncer.EXPECT().Stop().Return(nil)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockSyncer, mockAgent, mockChecker, store
}

func prepareDirect(t *testing.T) (
	*mock_monitor.MockMonitor, *mock_executor.MockExecutor,
	*checker.MockChecker, *mock_peermgr.MockPeerManager,
	*api.Server, storage.Storage,
) {
	mockCtl := gomock.NewController(t)
	mockMonitor := mock_monitor.NewMockMonitor(mockCtl)
	mockExecutor := mock_executor.NewMockExecutor(mockCtl)
	mockChecker := &checker.MockChecker{}
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)
	mockAppchainMgr := &appchain.Manager{}
	mockServer, err := api.NewServer(mockAppchainMgr, mockPeerMgr, &repo.Config{})
	require.Nil(t, err)

	mockPeerMgr.EXPECT().Start().Return(nil)
	mockPeerMgr.EXPECT().Stop().Return(nil)
	mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil)
	mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockChecker, mockPeerMgr, mockServer, store
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	ct := &pb.Content{
		SrcContractId: from,
		DstContractId: to,
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
		To:        to,
		Payload:   ibtppd,
		Index:     index,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
	}
}
