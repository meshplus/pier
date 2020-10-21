package exchanger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	network "github.com/meshplus/go-lightp2p"
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
	"github.com/meshplus/pier/internal/router/mock_router"
	"github.com/meshplus/pier/internal/syncer/mock_syncer"
	"github.com/stretchr/testify/require"
)

const (
	hash      = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	from      = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to        = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	assetTxID = "asset exchange id for test"
)

func TestStartRelay(t *testing.T) {
	mode := repo.RelayMode
	mockMonitor, mockExecutor, mockSyncer, mockAgent, mockChecker, store := prepareRelay(t)
	meta := &pb.Interchain{}

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
	receipt := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	bxhReceipt := &pb.Receipt{
		Status: pb.Receipt_SUCCESS,
		TxHash: types.NewHashByStr(hash),
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
	mockExchanger.handleIBTP(normalInIBTP)
	mockExchanger.handleIBTP(replayedInIBTP)

	// test for unhappy path, one normal indexed ibtp and will trigger getMissing with error
	unhappyPathMissedOutIBTPID := fmt.Sprintf("%s-%s-%d", from, to, 3)
	//unhappyPathOutIBTP := getIBTP(t, 4, pb.IBTP_INTERCHAIN)
	//outCh <- unhappyPathOutIBTP
	//
	mockMonitor.EXPECT().QueryIBTP(unhappyPathMissedOutIBTPID).Return(nil, fmt.Errorf("test error for query missing ibtp")).AnyTimes()

	// test for asset exchange ibtp
	signs := []byte("signs for asset exchange")
	mockAgent.EXPECT().GetAssetExchangeSigns(assetTxID).Return(signs, nil).AnyTimes()
	assetExchangeIBTP := getIBTP(t, 3, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	assetExchangeIBTP.Extra = []byte(assetTxID)
	mockExchanger.handleIBTP(assetExchangeIBTP)

	time.Sleep(500 * time.Microsecond)
	close(outCh)
	require.Nil(t, mockExchanger.Stop())
}

func TestStartDirect(t *testing.T) {
	mode := repo.DirectMode
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t)
	meta := &pb.Interchain{}

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithChecker(mockChecker), WithPeerMgr(mockPeerMgr),
		WithAPIServer(apiServer), WithStorage(store),
	)
	require.Nil(t, err)

	// test for happy path, one normal indexed ibtp and will trigger getMissing
	happyPathMissedOutIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	happyPathOutIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	receipt := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
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
	mockMonitor.EXPECT().QueryIBTP(happyPathMissedOutIBTP.ID()).Return(happyPathMissedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().HandleIBTP(gomock.Any()).Return(receipt).AnyTimes()
	mockExecutor.EXPECT().QueryLatestMeta().Return(inMeta)
	mockExecutor.EXPECT().QueryReceipt(to, uint64(1), gomock.Any()).Return(receipt, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), metaMsg).Return(retMetaMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(retMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	require.Nil(t, mockExchanger.Start())

	mockExchanger.handleNewConnection(to)

	outCh <- happyPathOutIBTP

	var stream network.Stream
	ibtpBytes, err := happyPathOutIBTP.Marshal()
	require.Nil(t, err)
	ibtpMsg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, ibtpBytes)
	mockExchanger.handleSendIBTPMessage(stream, ibtpMsg)

	ibtpGetMsg := peermgr.Message(peerMsg.Message_IBTP_GET, true, []byte(happyPathMissedOutIBTP.ID()))
	mockExchanger.handleGetIBTPMessage(stream, ibtpGetMsg)

	time.Sleep(1 * time.Second)
	// test for unhappy path, one normal indexed ibtp and will trigger getMissing with error
	unhappyPathMissedOutIBTPID := fmt.Sprintf("%s-%s-%d", from, to, 3)

	unhappyPathOutIBTP := getIBTP(t, 4, pb.IBTP_INTERCHAIN)
	outCh <- unhappyPathOutIBTP

	mockMonitor.EXPECT().QueryIBTP(unhappyPathMissedOutIBTPID).Return(nil, fmt.Errorf("test error for query missing ibtp")).AnyTimes()
	ibtpGetMsg = peermgr.Message(peerMsg.Message_IBTP_GET, true, []byte(unhappyPathMissedOutIBTPID))
	mockExchanger.handleGetIBTPMessage(stream, ibtpGetMsg)

	close(outCh)
	time.Sleep(500 * time.Microsecond)
	require.Nil(t, mockExchanger.Stop())
}

func TestStartUnionMode(t *testing.T) {
	mode := repo.UnionMode
	mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, mockAgent, store := prepareUnoin(t)
	meta := &pb.Interchain{}

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithPeerMgr(mockPeerMgr),
		WithRouter(mockRouter), WithStorage(store),
		WithAgent(mockAgent),
	)
	require.Nil(t, err)

	receipt := &pb.Receipt{
		TxHash: types.NewHashByStr(hash),
		Status: pb.Receipt_SUCCESS,
	}
	var stream network.Stream
	// mock ibtp for Message_ROUTER_IBTP_SEND
	ibtp := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	ibtpBytes, err := ibtp.Marshal()
	require.Nil(t, err)
	ibtpMsg := peermgr.Message(peerMsg.Message_ROUTER_IBTP_SEND, true, ibtpBytes)
	// mock getInterchainMsg for Message_ROUTER_INTERCHAIN_SEND
	interchainInfoMsg := peermgr.Message(peerMsg.Message_ROUTER_INTERCHAIN_SEND, true, []byte(from))
	interchainCounter := &pb.Interchain{
		InterchainCounter:    map[string]uint64{from: 1},
		ReceiptCounter:       map[string]uint64{from: 1},
		SourceReceiptCounter: map[string]uint64{from: 1},
	}
	signs := []byte("signs for ibtp in bitxhub")
	appchains := []*rpcx.Appchain{
		{
			ID:         from,
			Name:       "hpc",
			Validators: "validator for hpc",
			Status:     0,
			ChainType:  "hyperchain",
			Desc:       "appchain for test",
			PublicKey:  "",
		},
	}
	pierID := from
	icBytes, err := interchainCounter.Marshal()
	require.Nil(t, err)
	recoverACKMsg := peermgr.Message(peerMsg.Message_ACK, true, icBytes)

	mockAgent.EXPECT().SendIBTP(gomock.Any()).Return(receipt, nil).AnyTimes()
	mockAgent.EXPECT().GetInterchainById(from).Return(interchainCounter).AnyTimes()
	mockAgent.EXPECT().GetIBTPSigns(ibtp).Return(signs, nil).AnyTimes()
	mockAgent.EXPECT().GetAppchains().Return(appchains, nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().FindProviders(ibtp.To).Return(pierID, nil)
	mockPeerMgr.EXPECT().Send(pierID, gomock.Any()).Return(recoverACKMsg, nil)
	mockRouter.EXPECT().ExistAppchain(from).Return(true).AnyTimes()
	mockRouter.EXPECT().Route(ibtp).Return(nil).AnyTimes()
	mockRouter.EXPECT().AddAppchains(appchains).Return(nil).AnyTimes()
	require.Nil(t, mockExchanger.Start())

	// test Message_ROUTER_IBTP_SEND for peerMgr to handle msg from other union piers
	mockExchanger.handleRouterSendIBTPMessage(stream, ibtpMsg)

	// test Message_ROUTER_INTERCHAIN_SEND for peerMgr to handle msg from other union piers
	mockExchanger.handleRouterInterchain(stream, interchainInfoMsg)

	// test RegisterIBTPHandler for syncer to handle ibtp from bitxhub
	mockExchanger.handleUnionIBTP(ibtp)

	// test RegisterAppchainHandler for syncer to handle new appchain added to bitxhub
	mockExchanger.handleProviderAppchains()

	// test RegisterRecoverHandler for syncer to handle recover in bitxhub
	mockExchanger.handleRecover(ibtp)
	time.Sleep(1 * time.Second)
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

func prepareUnoin(t *testing.T) (
	*mock_monitor.MockMonitor, *mock_executor.MockExecutor,
	*mock_syncer.MockSyncer, *mock_peermgr.MockPeerManager,
	*mock_router.MockRouter, *mock_agent.MockAgent, storage.Storage,
) {
	mockCtl := gomock.NewController(t)
	mockMonitor := mock_monitor.NewMockMonitor(mockCtl)
	mockExecutor := mock_executor.NewMockExecutor(mockCtl)
	mockSyncer := mock_syncer.NewMockSyncer(mockCtl)
	mockRouter := mock_router.NewMockRouter(mockCtl)
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)
	mockAgent := mock_agent.NewMockAgent(mockCtl)

	mockPeerMgr.EXPECT().Start().Return(nil)
	mockPeerMgr.EXPECT().Stop().Return(nil)
	mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil)
	mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().Start().Return(nil)
	mockSyncer.EXPECT().Stop().Return(nil)
	mockSyncer.EXPECT().RegisterIBTPHandler(gomock.Any()).Return(nil)
	mockSyncer.EXPECT().RegisterAppchainHandler(gomock.Any()).Return(nil)
	mockSyncer.EXPECT().RegisterRecoverHandler(gomock.Any()).Return(nil)
	mockRouter.EXPECT().Start().Return(nil)
	mockRouter.EXPECT().Stop().Return(nil)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, mockAgent, store
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
