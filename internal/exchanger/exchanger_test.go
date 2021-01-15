package exchanger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	ecdsa2 "github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/api"
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
	from      = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to        = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	assetTxID = "asset exchange id for test"
)

func TestStartRelay(t *testing.T) {
	mode := repo.RelayMode
	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	meta := &pb.Interchain{}

	// mock one missing ibtp and one normal listened ibtp
	missedOutIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	normalOutIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	missedInIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	receipt := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	outCh := make(chan *pb.IBTP, 1)
	outCh <- normalOutIBTP

	inCh := make(chan *pb.IBTP)

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta)
	mockMonitor.EXPECT().QueryIBTP(missedOutIBTP.ID()).Return(missedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(make(map[string]uint64)).AnyTimes()
	mockExecutor.EXPECT().QueryIBTPReceipt(to, uint64(1), gomock.Any()).Return(receipt, nil).AnyTimes()
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).Return(receipt, nil).AnyTimes()
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	mockSyncer.EXPECT().QueryIBTP(receipt.ID()).Return(missedInIBTP, nil)
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)
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
	mockSyncer.EXPECT().GetAssetExchangeSigns(assetTxID).Return(signs, nil).AnyTimes()
	assetExchangeIBTP := getIBTP(t, 3, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	assetExchangeIBTP.Extra = []byte(assetTxID)
	mockExchanger.handleIBTP(assetExchangeIBTP)

	time.Sleep(500 * time.Microsecond)
	close(outCh)
	close(inCh)
	require.Nil(t, mockExchanger.Stop())
}

func TestStartDirect(t *testing.T) {
	mode := repo.DirectMode
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t)
	meta := &pb.Interchain{}



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

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta)
	mockMonitor.EXPECT().QueryIBTP(happyPathMissedOutIBTP.ID()).Return(happyPathMissedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).Return(receipt, nil).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(make(map[string]uint64)).AnyTimes()
	mockExecutor.EXPECT().QueryIBTPReceipt(to, uint64(1), gomock.Any()).Return(receipt, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), metaMsg).Return(retMetaMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(retMsg, nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSend(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()


	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithChecker(mockChecker), WithPeerMgr(mockPeerMgr),
		WithAPIServer(apiServer), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)
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
	time.Sleep(500 * time.Millisecond)
	require.Nil(t, mockExchanger.Stop())
}

func TestStartUnionMode(t *testing.T) {
	mode := repo.UnionMode
	mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, store := prepareUnoin(t)
	meta := &pb.Interchain{}

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

	inCh := make(chan *pb.IBTP)

	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().GetInterchainById(from).Return(interchainCounter).AnyTimes()
	mockSyncer.EXPECT().GetIBTPSigns(ibtp).Return(signs, nil).AnyTimes()
	mockSyncer.EXPECT().GetAppchains().Return(appchains, nil).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(map[string]uint64{to: 1}).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(make(map[string]uint64)).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().FindProviders(ibtp.To).Return(pierID, nil)
	mockPeerMgr.EXPECT().Send(pierID, gomock.Any()).Return(recoverACKMsg, nil)
	mockRouter.EXPECT().ExistAppchain(from).Return(true).AnyTimes()
	mockRouter.EXPECT().Route(ibtp).Return(nil).AnyTimes()
	mockRouter.EXPECT().AddAppchains(appchains).Return(nil).AnyTimes()


	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithPeerMgr(mockPeerMgr),
		WithRouter(mockRouter), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)
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
	close(inCh)
	require.Nil(t, mockExchanger.Stop())
}

func prepareRelay(t *testing.T) (
	*mock_monitor.MockMonitor, *mock_executor.MockExecutor,
	*mock_syncer.MockSyncer,
	*checker.MockChecker, storage.Storage,
) {
	mockCtl := gomock.NewController(t)
	mockMonitor := mock_monitor.NewMockMonitor(mockCtl)
	mockExecutor := mock_executor.NewMockExecutor(mockCtl)
	mockSyncer := mock_syncer.NewMockSyncer(mockCtl)
	mockChecker := &checker.MockChecker{}

	mockSyncer.EXPECT().Start().Return(nil)
	mockSyncer.EXPECT().Stop().Return(nil)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockSyncer, mockChecker, store
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
	mockServer, err := api.NewServer(mockAppchainMgr, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
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
	*mock_router.MockRouter, storage.Storage,
) {
	mockCtl := gomock.NewController(t)
	mockMonitor := mock_monitor.NewMockMonitor(mockCtl)
	mockExecutor := mock_executor.NewMockExecutor(mockCtl)
	mockSyncer := mock_syncer.NewMockSyncer(mockCtl)
	mockRouter := mock_router.NewMockRouter(mockCtl)
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)

	mockPeerMgr.EXPECT().Start().Return(nil)
	mockPeerMgr.EXPECT().Stop().Return(nil)
	mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil)
	mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().Start().Return(nil)
	mockSyncer.EXPECT().Stop().Return(nil)
	mockSyncer.EXPECT().RegisterAppchainHandler(gomock.Any()).Return(nil)
	mockSyncer.EXPECT().RegisterRecoverHandler(gomock.Any()).Return(nil)
	mockRouter.EXPECT().Start().Return(nil)
	mockRouter.EXPECT().Stop().Return(nil)

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, store
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

func getIBTPWithFromTo(t *testing.T, index uint64, typ pb.IBTP_Type, from, to string) *pb.IBTP {
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

func getIBTPs(t *testing.T, start, size uint64, typ pb.IBTP_Type, from, to string) []*pb.IBTP {
	var ibtps []*pb.IBTP

	for i := start; i < start+size; i++ {
		ibtps = append(ibtps, getIBTPWithFromTo(t, i, typ, from, to))
	}

	return ibtps
}

func TestWithPeerMgr(t *testing.T) {
	ibtpSize := uint64(110)
	mode := "direct"
	mockMonitor1, mockExecutor1, mockChecker1, _, apiServer1, store1 := prepareDirect(t)
	mockMonitor2, mockExecutor2, mockChecker2, _, apiServer2, store2 := prepareDirect(t)
	meta := &pb.Interchain{}

	nodeKeys, privKeys, config, addrs := genKeysAndConfig(t, 2)

	swarm1, err := peermgr.New(config, nodeKeys[0], privKeys[0], 0, log.NewWithModule("swarm"))
	require.Nil(t, err)

	swarm2, err := peermgr.New(config, nodeKeys[1], privKeys[1], 0, log.NewWithModule("swarm"))
	require.Nil(t, err)

	inMeta := make(map[string]uint64)
	mockExecutor1.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor2.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor1.EXPECT().QueryCallbackMeta().Return(make(map[string]uint64)).AnyTimes()
	mockExecutor2.EXPECT().QueryCallbackMeta().Return(make(map[string]uint64)).AnyTimes()

	mockExchanger1, err := New(mode, addrs[0], meta,
		WithMonitor(mockMonitor1), WithExecutor(mockExecutor1),
		WithChecker(mockChecker1), WithPeerMgr(swarm1),
		WithAPIServer(apiServer1), WithStorage(store1),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	mockExchanger2, err := New(mode, addrs[1], meta,
		WithMonitor(mockMonitor2), WithExecutor(mockExecutor2),
		WithChecker(mockChecker2), WithPeerMgr(swarm2),
		WithAPIServer(apiServer2), WithStorage(store2),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	normalOutIBTPs := getIBTPs(t, 1, ibtpSize, pb.IBTP_INTERCHAIN, addrs[0], addrs[1])
	normalReceipts := getIBTPs(t, 1, ibtpSize, pb.IBTP_RECEIPT_SUCCESS, addrs[0], addrs[1])
	outCh := make(chan *pb.IBTP, 1000)

	outMeta := make(map[string]uint64)

	mockMonitor1.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
	mockMonitor1.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	for _, ibtp := range normalOutIBTPs {
		//	mockExecutor1.EXPECT().HandleIBTP(ibtp).Return(normalReceipts[i]).AnyTimes()
		outCh <- ibtp
	}

	mockMonitor2.EXPECT().ListenIBTP().Return(make(chan *pb.IBTP)).AnyTimes()
	mockMonitor2.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	for i, ibtp := range normalOutIBTPs {
		mockExecutor2.EXPECT().ExecuteIBTP(ibtp).Return(normalReceipts[i], nil).AnyTimes()
		//outCh <- ibtp
	}


	go mockExchanger1.Start()
	go mockExchanger2.Start()

	time.Sleep(5 * time.Second)
}

func genKeysAndConfig(t *testing.T, peerCnt int) ([]crypto.PrivateKey, []crypto.PrivateKey, *repo.Config, []string) {
	var nodeKeys []crypto.PrivateKey
	var privKeys []crypto.PrivateKey
	var peers []string
	var addrs []string
	port := 5001

	for i := 0; i < peerCnt; i++ {
		key, err := asym.GenerateKeyPair(crypto.ECDSA_P256)
		require.Nil(t, err)
		nodeKeys = append(nodeKeys, key)

		libp2pKey, err := convertToLibp2pPrivKey(key)
		require.Nil(t, err)

		id, err := peer2.IDFromPrivateKey(libp2pKey)
		require.Nil(t, err)

		peer := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, id)
		peers = append(peers, peer)

		privKey, err := asym.GenerateKeyPair(crypto.Secp256k1)
		require.Nil(t, err)

		privKeys = append(privKeys, privKey)

		addr, err := privKey.PublicKey().Address()
		require.Nil(t, err)

		addrs = append(addrs, addr.String())

		port++
	}

	config := &repo.Config{}
	config.Mode.Type = repo.DirectMode
	config.Mode.Direct.Peers = peers

	return nodeKeys, privKeys, config, addrs
}

func convertToLibp2pPrivKey(privateKey crypto.PrivateKey) (crypto2.PrivKey, error) {
	ecdsaPrivKey, ok := privateKey.(*ecdsa2.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("convert to libp2p private key: not ecdsa private key")
	}

	libp2pPrivKey, _, err := crypto2.ECDSAKeyPairFromKey(ecdsaPrivKey.K)
	if err != nil {
		return nil, err
	}

	return libp2pPrivKey, nil
}
