package exchanger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
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
	"github.com/meshplus/pier/pkg/model"
	"github.com/stretchr/testify/require"
)

const (
	from      = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to        = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	pierID    = "0x892eedc032948be00722038a29f2a90d0e05352f"
	assetTxID = "asset exchange id for test"
)

var errorUnhappy = fmt.Errorf("nil")

func TestStartRelay(t *testing.T) {
	testNormalStartRelay(t)
	testRecoverMissingCallbackStartRelay(t)
	testRecoverErrorStartRelay(t)
	testApplyInterchain(t)
	testApplyReceipt(t)
}

func testNormalStartRelay(t *testing.T) {
	mode := repo.RelayMode
	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	meta := &pb.Interchain{}

	outCh := make(chan *pb.IBTP)
	inCh := make(chan *model.WrappedIBTP)

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	callbackMeta := make(map[string]uint64)
	callbackMeta[to] = 1

	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
	mockSyncer.EXPECT().RegisterRollbackHandler(gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().Start().Return(nil)
	mockSyncer.EXPECT().Stop().Return(nil)
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()

	ibtps, ibtpM := genIBTPs(t, 100, pb.IBTP_INTERCHAIN)
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).
		DoAndReturn(func(id string) (*pb.IBTP, bool, error) {
			return ibtpM[id], true, nil
		}).AnyTimes()

	mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).
		DoAndReturn(func(originalIBTP *pb.IBTP) (*pb.IBTP, error) {
			ibtp := getIBTP(t, originalIBTP.Index, pb.IBTP_RECEIPT_SUCCESS)
			return ibtp, nil
		}).AnyTimes()
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).
		DoAndReturn(func(wIbtp *model.WrappedIBTP) (*pb.IBTP, error) {
			receipt := getIBTP(t, wIbtp.Ibtp.Index, pb.IBTP_RECEIPT_SUCCESS)
			return receipt, nil
		}).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)

	mntIbtps, mntIbtpM := genIBTPs(t, 100, pb.IBTP_INTERCHAIN)
	mockMonitor.EXPECT().QueryIBTP(gomock.Any()).DoAndReturn(func(id string) (*pb.IBTP, error) {
		return mntIbtpM[id], nil
	}).AnyTimes()

	require.Nil(t, err)
	require.Nil(t, mockExchanger.Start())

	testRelayIBTPFromSyncer(t, inCh, ibtps, mockExchanger)

	testRelayIBTPFromMnt(t, outCh, mntIbtps, mockExchanger)

	// test for asset exchange ibtp
	signs := []byte("signs for asset exchange")
	mockSyncer.EXPECT().GetAssetExchangeSigns(assetTxID).Return(signs, nil).AnyTimes()
	assetExchangeIBTP := getIBTP(t, 3, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	assetExchangeIBTP.Extra = []byte(assetTxID)
	mockExchanger.handleIBTP(&model.WrappedIBTP{
		Ibtp:    assetExchangeIBTP,
		IsValid: true,
	})

	time.Sleep(500 * time.Microsecond)
	close(outCh)
	close(inCh)
	require.Nil(t, mockExchanger.Stop())
}

func testApplyInterchain(t *testing.T) {
	mode := repo.RelayMode
	meta := &pb.Interchain{}
	outMeta := make(map[string]uint64)
	inMeta := make(map[string]uint64)
	callbackMeta := make(map[string]uint64)

	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)

	inCh := make(chan *model.WrappedIBTP, 2)
	signs := []byte("signs for asset exchange")
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	go mockExchanger.listenAndSendIBTPFromSyncer()

	mockExchanger.executorCounter[to] = 1
	interchainIBTP1 := getIBTP(t, 1, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	inCh <- &model.WrappedIBTP{Ibtp: interchainIBTP1, IsValid: true}

	interchainIBTP2 := getIBTP(t, 2, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	interchainIBTP2.Extra = []byte(assetTxID)
	badInterchainIBTP3 := getIBTP(t, 3, 7)
	inCh <- &model.WrappedIBTP{Ibtp: badInterchainIBTP3, IsValid: false}
	interchainIBTP3 := getIBTP(t, 3, pb.IBTP_ASSET_EXCHANGE_REDEEM)
	interchainIBTP3.Extra = []byte(assetTxID)
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(nil, false, fmt.Errorf("query ibtp error"))
	inCh <- &model.WrappedIBTP{Ibtp: interchainIBTP3, IsValid: true}

	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(interchainIBTP2, true, nil)
	mockSyncer.EXPECT().GetAssetExchangeSigns(gomock.Any()).Return(signs, nil).AnyTimes()
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(fmt.Errorf("send ibtp error"))
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil)
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).Return(&pb.IBTP{}, nil).AnyTimes()
	inCh <- &model.WrappedIBTP{Ibtp: interchainIBTP3, IsValid: true}

	time.Sleep(2 * time.Second)
	require.Equal(t, uint64(3), mockExchanger.executorCounter[from])
}

func testApplyReceipt(t *testing.T) {
	mode := repo.RelayMode
	meta := &pb.Interchain{}
	outMeta := make(map[string]uint64)
	//outMeta[to] = 1
	inMeta := make(map[string]uint64)
	//inMeta[to] = 1
	callbackMeta := make(map[string]uint64)

	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)

	inCh := make(chan *model.WrappedIBTP, 2)
	//signs := []byte("signs for asset exchange")
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	go mockExchanger.listenAndSendIBTPFromSyncer()

	receiptIBTP1 := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	mockExchanger.callbackCounter[to] = 1
	inCh <- &model.WrappedIBTP{Ibtp: receiptIBTP1, IsValid: true}
	receiptIBTP3 := getIBTP(t, 3, pb.IBTP_RECEIPT_SUCCESS)
	receiptIBTP2 := getIBTP(t, 2, pb.IBTP_RECEIPT_SUCCESS)
	receiptIBTP2.Extra = []byte(assetTxID)
	//mockSyncer.EXPECT().GetAssetExchangeSigns(assetTxID).Return(nil, fmt.Errorf("get signs error"))
	//mockSyncer.EXPECT().GetAssetExchangeSigns(gomock.Any()).Return(signs, nil)

	wReceiptIBTP2 := &model.WrappedIBTP{Ibtp: receiptIBTP2, IsValid: true}
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).Return(nil, nil).AnyTimes()
	inCh <- &model.WrappedIBTP{Ibtp: receiptIBTP3, IsValid: true}
	inCh <- wReceiptIBTP2
	time.Sleep(2 * time.Second)
	require.Equal(t, uint64(3), mockExchanger.callbackCounter[from])
}

func testRecoverMissingCallbackStartRelay(t *testing.T) {
	mode := repo.RelayMode
	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	meta := &pb.Interchain{}

	outMeta := make(map[string]uint64)
	inMeta := make(map[string]uint64)
	callbackMeta := make(map[string]uint64)

	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)
	mockSyncer.EXPECT().RegisterRollbackHandler(gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().Start().Return(nil).AnyTimes()

	// mock recover error
	// 1. test for recover missing callback
	mockExchanger.interchainCounter[to] = 1
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(nil, false, fmt.Errorf("query ibtp error"))
	failedCallback := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(failedCallback, false, nil)

	outCh := make(chan *pb.IBTP)
	inCh := make(chan *model.WrappedIBTP)
	mockMonitor.EXPECT().ListenIBTP().Return(outCh)
	mockSyncer.EXPECT().ListenIBTP().Return(inCh)
	require.Nil(t, mockExchanger.Start())
	time.Sleep(100 * time.Millisecond)
}

func testRecoverErrorStartRelay(t *testing.T) {
	mode := repo.RelayMode
	mockMonitor, mockExecutor, mockSyncer, mockChecker, store := prepareRelay(t)
	meta := &pb.Interchain{}

	outCh := make(chan *pb.IBTP)
	inCh := make(chan *model.WrappedIBTP)

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	callbackMeta := make(map[string]uint64)

	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()

	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
		WithChecker(mockChecker), WithStorage(store),
	)
	require.Nil(t, err)

	// test for RegisterRollbackHandler error
	registerError := fmt.Errorf("RegisterRollbackHandler errr")
	mockSyncer.EXPECT().RegisterRollbackHandler(gomock.Any()).Return(registerError)
	require.Equal(t, true, errors.Is(mockExchanger.Start(), registerError))

	mockSyncer.EXPECT().RegisterRollbackHandler(gomock.Any()).Return(nil).AnyTimes()
	// test for syncer start error
	syncerError := fmt.Errorf("start syncer failed")
	mockSyncer.EXPECT().Start().Return(syncerError)
	require.Equal(t, true, errors.Is(mockExchanger.Start(), syncerError))

	mockSyncer.EXPECT().Start().Return(nil).AnyTimes()

	// mock recover error
	// 1. test for recover unsent interchain ibtp failure and panic
	mockMonitor.EXPECT().QueryIBTP(gomock.Any()).Return(nil, fmt.Errorf("query ibtp error"))
	require.Panics(t, func() { mockExchanger.Start() })

	mockMonitor.EXPECT().QueryIBTP(gomock.Any()).Return(nil, nil)
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(fmt.Errorf("send ibtp receipt error")).MaxTimes(2)
	require.Panics(t, func() { mockExchanger.Start() })

	// 2. test for recover missing receipt
	outMeta[to] = 0
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(nil, false, fmt.Errorf("query ibtp error")).MaxTimes(2)
	mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(&pb.IBTP{}, false, nil).MaxTimes(5)
	mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).Return(nil, fmt.Errorf("query receipt error"))
	mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).Return(&pb.IBTP{}, nil).MaxTimes(4)
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(fmt.Errorf("send ibtp receipt error"))
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil)

	outCh := make(chan *pb.IBTP)
	inCh := make(chan *model.WrappedIBTP)
	mockMonitor.EXPECT().ListenIBTP().Return(outCh)
	mockSyncer.EXPECT().ListenIBTP().Return(inCh)
	require.Nil(t, mockExchanger.Start())
	time.Sleep(100 * time.Millisecond)
}

func testRelayIBTPFromMnt(t *testing.T, outCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			outCh <- ibtp
		}
	}
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.interchainCounter[ibtps[0].To])
}

func testRelayIBTPFromSyncer(t *testing.T, inCh chan *model.WrappedIBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			inCh <- &model.WrappedIBTP{Ibtp: ibtp, IsValid: true}
		}
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.executorCounter[ibtps[0].From])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS)
	for _, receipt := range ibtpReceipts {
		inCh <- &model.WrappedIBTP{Ibtp: receipt, IsValid: true}
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.callbackCounter[ibtps[0].To])
}

func genIBTPs(t *testing.T, count int, typ pb.IBTP_Type) ([]*pb.IBTP, map[string]*pb.IBTP) {
	ibtps := make([]*pb.IBTP, 0, count)
	ibtpM := make(map[string]*pb.IBTP, count)
	for i := 1; i <= count; i++ {
		ibtp := getIBTP(t, uint64(i), typ)
		ibtps = append(ibtps, ibtp)
		ibtpM[ibtp.ID()] = ibtp
	}
	return ibtps, ibtpM
}

func TestStartDirect(t *testing.T) {
	testNormalDirect(t)
	testErrorStartStopDirect(t)
	testErrorDirect(t)
}

func testErrorDirect(t *testing.T) {
	mode := repo.DirectMode
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t, false)
	meta := &pb.Interchain{}

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	callbackMeta := make(map[string]uint64)

	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithChecker(mockChecker), WithPeerMgr(mockPeerMgr),
		WithAPIServer(apiServer), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	var stream network.Stream
	// test wrong marshaled ibtp bytes
	badIbtpMsg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, []byte("bad ibtp msg bytes"))
	mockExchanger.handleSendIBTPMessage(stream, badIbtpMsg)

	// test for wrong index ibtp
	ignoredIBTP := &model.WrappedIBTP{
		Ibtp:    getIBTP(t, 1, pb.IBTP_INTERCHAIN),
		IsValid: true,
	}
	ibtpBytes, err := json.Marshal(ignoredIBTP)
	require.Nil(t, err)
	ignoredIBTPMsg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, ibtpBytes)
	mockExchanger.handleSendIBTPMessage(stream, ignoredIBTPMsg)
	time.Sleep(100 * time.Millisecond)

	receiptIBTP3 := getIBTP(t, 3, pb.IBTP_RECEIPT_SUCCESS)
	ibtp3 := &model.WrappedIBTP{
		Ibtp:    getIBTP(t, 3, pb.IBTP_INTERCHAIN),
		IsValid: true,
	}
	ibtpBytes, err = json.Marshal(ibtp3)
	require.Nil(t, err)
	outIBTPMsg := peermgr.Message(peerMsg.Message_IBTP_SEND, true, ibtpBytes)
	mockExecutor.EXPECT().ExecuteIBTP(ibtp3).Return(receiptIBTP3, nil)
	mockPeerMgr.EXPECT().AsyncSend(from, gomock.Any()).Return(fmt.Errorf("async send ibtp fail"))
	mockExchanger.handleSendIBTPMessage(stream, outIBTPMsg)
	time.Sleep(100 * time.Millisecond)

	// test for multiple ibtp
	ibtp2 := &model.WrappedIBTP{
		Ibtp:    getIBTP(t, 2, pb.IBTP_INTERCHAIN),
		IsValid: true,
	}
	ibtpBytes, err = json.Marshal(ibtp2)
	require.Nil(t, err)
	outIBTPMsg = peermgr.Message(peerMsg.Message_IBTP_SEND, true, ibtpBytes)
	mockExecutor.EXPECT().ExecuteIBTP(ibtp2).Return(nil, nil)
	mockExchanger.handleSendIBTPMessage(stream, outIBTPMsg)
	mockPeerMgr.EXPECT().AsyncSend(from, gomock.Any()).Return(fmt.Errorf("async send ibtp fail"))
	time.Sleep(100 * time.Millisecond)
}

func testNormalDirect(t *testing.T) {
	mode := repo.DirectMode
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t, true)
	meta := &pb.Interchain{}

	// test for happy path, one normal indexed ibtp and will trigger getMissing
	happyPathMissedOutIBTP := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	happyPathOutIBTP := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	receipt := getIBTP(t, 1, pb.IBTP_ASSET_EXCHANGE_RECEIPT)
	receipt1 := getIBTP(t, 2, pb.IBTP_ASSET_EXCHANGE_RECEIPT)
	outCh := make(chan *pb.IBTP, 1)

	receiptBytes, err := receipt.Marshal()
	require.Nil(t, err)

	receipt1Bytes, err := receipt1.Marshal()
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
	ret1Msg := peermgr.Message(peerMsg.Message_ACK, true, receipt1Bytes)

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1

	callbackMeta := make(map[string]uint64)

	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).MaxTimes(2)
	mockMonitor.EXPECT().QueryIBTP(happyPathMissedOutIBTP.ID()).Return(happyPathMissedOutIBTP, nil).AnyTimes()
	mockExecutor.EXPECT().ExecuteIBTP(gomock.Any()).Return(receipt, nil).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
	mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).Return(receipt, nil).AnyTimes()
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

	mockExchanger.handleSendIBTPReceiptMessage(stream, ret1Msg)
	mockExchanger.handleSendIBTPReceiptMessage(stream, retMsg)

	getInterchainMsg := peermgr.Message(peerMsg.Message_INTERCHAIN_META_GET, true, []byte(to))
	mockExchanger.handleGetInterchainMessage(stream, getInterchainMsg)

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

func testErrorStartStopDirect(t *testing.T) {
	mode := repo.DirectMode
	mockMonitor, mockExecutor, mockChecker, mockPeerMgr, apiServer, store := prepareDirect(t, false)
	meta := &pb.Interchain{}

	outMeta := make(map[string]uint64)
	outMeta[to] = 1
	inMeta := make(map[string]uint64)
	inMeta[to] = 1
	callbackMeta := make(map[string]uint64)

	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
	mockExchanger, err := New(mode, from, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithChecker(mockChecker), WithPeerMgr(mockPeerMgr),
		WithAPIServer(apiServer), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	startError := fmt.Errorf("start direct mode exchanger error")
	mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithDirectMode(), startError))
	mockExchanger.apiServer.Stop()
	mockServer, err := api.NewServer(&appchain.Manager{}, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
	require.Nil(t, err)
	mockExchanger.apiServer = mockServer

	mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil).AnyTimes()
	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_INTERCHAIN_META_GET, gomock.Any()).Return(startError)
	//require.Equal(t, true, errors.Is(mockExchanger.startWithDirectMode(), startError))
	//mockExchanger.apiServer.Stop()
	//mockServer, err = api.NewServer(&appchain.Manager{}, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
	//require.Nil(t, err)
	//mockExchanger.apiServer = mockServer
	//
	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_INTERCHAIN_META_GET, gomock.Any()).Return(nil).MaxTimes(5)
	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_IBTP_SEND, gomock.Any()).Return(startError)
	//require.Equal(t, true, errors.Is(mockExchanger.startWithDirectMode(), startError))
	//mockExchanger.apiServer.Stop()
	//mockServer, err = api.NewServer(&appchain.Manager{}, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
	//require.Nil(t, err)
	//mockExchanger.apiServer = mockServer
	//
	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_IBTP_SEND, gomock.Any()).Return(nil).MaxTimes(5)
	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_IBTP_RECEIPT_SEND, gomock.Any()).Return(startError)
	//require.Equal(t, true, errors.Is(mockExchanger.startWithDirectMode(), startError))
	//mockExchanger.apiServer.Stop()
	//mockServer, err = api.NewServer(&appchain.Manager{}, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
	//require.Nil(t, err)
	//mockExchanger.apiServer = mockServer

	//mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_IBTP_RECEIPT_SEND, gomock.Any()).Return(nil).MaxTimes(5)
	mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().Start().Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithDirectMode(), startError))
	mockExchanger.apiServer.Stop()
	mockServer, err = api.NewServer(&appchain.Manager{}, mockPeerMgr, &repo.Config{}, log.NewWithModule("api"))
	require.Nil(t, err)
	mockExchanger.apiServer = mockServer

	// mock stop direct exchanger
	stopError := fmt.Errorf("stop exchanger error")
	mockPeerMgr.EXPECT().Stop().Return(stopError)
	require.Equal(t, true, errors.Is(mockExchanger.Stop(), stopError))
}

func TestStartUnionMode(t *testing.T) {
	testUnionMode(from, t)
	testUnionMode(to, t)
	testUnionIBTP(t)
	testUnionStartAndStop(t)
}

func testUnionMode(pierID string, t *testing.T) {
	mode := repo.UnionMode
	mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, store := prepareUnoin(t, true)
	meta := &pb.Interchain{}

	var stream network.Stream
	// mock ibtp for Message_ROUTER_IBTP_SEND
	ibtp := &model.WrappedIBTP{
		Ibtp:    getIBTPWithFromTo(t, 1, pb.IBTP_INTERCHAIN, pierID, to),
		IsValid: true,
	}
	ibtpBytes, err := json.Marshal(ibtp)
	require.Nil(t, err)
	ibtpMsg := peermgr.Message(peerMsg.Message_ROUTER_IBTP_SEND, true, ibtpBytes)
	// mock getInterchainMsg for Message_ROUTER_INTERCHAIN_SEND
	interchainInfoMsg := peermgr.Message(peerMsg.Message_ROUTER_INTERCHAIN_SEND, true, []byte(pierID))
	interchainCounter := &pb.Interchain{
		InterchainCounter:    map[string]uint64{pierID: 1},
		ReceiptCounter:       map[string]uint64{pierID: 1},
		SourceReceiptCounter: map[string]uint64{pierID: 1},
	}
	signs := []byte("signs for ibtp in bitxhub")
	appchains := []*rpcx.Appchain{
		{
			ID:         pierID,
			Name:       "hpc",
			Validators: "validator for hpc",
			Status:     appchainmgr.AppchainAvailable,
			ChainType:  "hyperchain",
			Desc:       "appchain for test",
			PublicKey:  "",
		},
	}

	icBytes, err := interchainCounter.Marshal()
	require.Nil(t, err)
	recoverACKMsg := peermgr.Message(peerMsg.Message_ACK, true, icBytes)

	inCh := make(chan *model.WrappedIBTP)

	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().GetInterchainById(pierID).Return(interchainCounter).AnyTimes()
	mockSyncer.EXPECT().GetIBTPSigns(ibtp).Return(signs, nil).AnyTimes()
	mockSyncer.EXPECT().GetAppchains().Return(appchains, nil).AnyTimes()
	mockExecutor.EXPECT().QueryInterchainMeta().Return(map[string]uint64{to: 1}).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(map[string]uint64{to: 1}).AnyTimes()
	mockExecutor.EXPECT().ExecuteIBTP(ibtp).Return(nil, nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().FindProviders(ibtp.Ibtp.To).Return(pierID, nil)
	mockPeerMgr.EXPECT().Send(pierID, gomock.Any()).Return(recoverACKMsg, nil)
	mockRouter.EXPECT().ExistAppchain(pierID).Return(true).AnyTimes()
	mockRouter.EXPECT().Route(ibtp).Return(nil).AnyTimes()
	mockRouter.EXPECT().AddAppchains(appchains).Return(nil).AnyTimes()

	mockExchanger, err := New(mode, pierID, meta,
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
	mockExchanger.handleRecover(ibtp.Ibtp)
	time.Sleep(1 * time.Second)
	close(inCh)
	require.Nil(t, mockExchanger.Stop())
}

func testUnionStartAndStop(t *testing.T) {
	mode := repo.UnionMode
	mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, store := prepareUnoin(t, false)
	meta := &pb.Interchain{}
	mockExecutor.EXPECT().QueryInterchainMeta().Return(map[string]uint64{to: 1}).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(map[string]uint64{to: 1}).AnyTimes()

	mockExchanger, err := New(mode, pierID, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithPeerMgr(mockPeerMgr),
		WithRouter(mockRouter), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	inCh := make(chan *model.WrappedIBTP)
	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
	// mock exchanger start error
	startError := fmt.Errorf("start unoin exchanger error")
	mockPeerMgr.EXPECT().Start().Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockPeerMgr.EXPECT().Start().Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_ROUTER_IBTP_SEND, gomock.Any()).Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_ROUTER_IBTP_SEND, gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_ROUTER_INTERCHAIN_SEND, gomock.Any()).Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockPeerMgr.EXPECT().RegisterMsgHandler(peerMsg.Message_ROUTER_INTERCHAIN_SEND, gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().RegisterAppchainHandler(gomock.Any()).Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockSyncer.EXPECT().RegisterAppchainHandler(gomock.Any()).Return(nil).AnyTimes()
	mockSyncer.EXPECT().RegisterRecoverHandler(gomock.Any()).Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockSyncer.EXPECT().RegisterRecoverHandler(gomock.Any()).Return(nil).AnyTimes()
	mockRouter.EXPECT().Start().Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	mockRouter.EXPECT().Start().Return(nil).AnyTimes()
	mockSyncer.EXPECT().Start().Return(startError)
	require.Equal(t, true, errors.Is(mockExchanger.startWithUnionMode(), startError))

	// mock exchanger stop error
	stopError := fmt.Errorf("stop exchanger error")
	mockSyncer.EXPECT().Stop().Return(stopError)
	require.Equal(t, true, errors.Is(mockExchanger.Stop(), stopError))

	mockSyncer.EXPECT().Stop().Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().Stop().Return(stopError)
	require.Equal(t, true, errors.Is(mockExchanger.Stop(), stopError))

	mockPeerMgr.EXPECT().Stop().Return(nil).AnyTimes()
	mockRouter.EXPECT().Stop().Return(stopError)
	require.Equal(t, true, errors.Is(mockExchanger.Stop(), stopError))
}

func testUnionIBTP(t *testing.T) {
	mode := repo.UnionMode
	mockMonitor, mockExecutor, mockSyncer, mockPeerMgr, mockRouter, store := prepareUnoin(t, false)
	meta := &pb.Interchain{}
	mockExecutor.EXPECT().QueryInterchainMeta().Return(map[string]uint64{to: 1}).AnyTimes()
	mockExecutor.EXPECT().QueryCallbackMeta().Return(map[string]uint64{to: 1}).AnyTimes()

	mockExchanger, err := New(mode, pierID, meta,
		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
		WithSyncer(mockSyncer), WithPeerMgr(mockPeerMgr),
		WithRouter(mockRouter), WithStorage(store),
		WithLogger(log.NewWithModule("exchanger")),
	)
	require.Nil(t, err)

	signs := []byte("signs for asset exchange")
	unoinIBTP := getIBTPWithFromTo(t, 1, pb.IBTP_INTERCHAIN, pierID, to)
	mockSyncer.EXPECT().GetIBTPSigns(unoinIBTP).Return(nil, fmt.Errorf("get ibtp signs error"))
	mockSyncer.EXPECT().GetIBTPSigns(unoinIBTP).Return(signs, nil)

	mockRouter.EXPECT().Route(unoinIBTP).Return(fmt.Errorf("route ibtp error"))
	mockRouter.EXPECT().Route(unoinIBTP).Return(nil)
	mockExchanger.handleUnionIBTP(&model.WrappedIBTP{
		Ibtp:    unoinIBTP,
		IsValid: true,
	})
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

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockSyncer, mockChecker, store
}

func prepareDirect(t *testing.T, isNormal bool) (
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

	if isNormal {
		mockPeerMgr.EXPECT().Start().Return(nil)
		mockPeerMgr.EXPECT().Stop().Return(nil)
		mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil)
		mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	}

	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	return mockMonitor, mockExecutor, mockChecker, mockPeerMgr, mockServer, store
}

func prepareUnoin(t *testing.T, isNormal bool) (
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

	if isNormal {
		mockPeerMgr.EXPECT().Start().Return(nil)
		mockPeerMgr.EXPECT().Stop().Return(nil)
		//mockPeerMgr.EXPECT().RegisterConnectHandler(gomock.Any()).Return(nil)
		mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		mockSyncer.EXPECT().Start().Return(nil)
		mockSyncer.EXPECT().Stop().Return(nil)
		mockSyncer.EXPECT().RegisterAppchainHandler(gomock.Any()).Return(nil)
		mockSyncer.EXPECT().RegisterRecoverHandler(gomock.Any()).Return(nil)
		mockRouter.EXPECT().Start().Return(nil)
		mockRouter.EXPECT().Stop().Return(nil)
	}

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
	mockMonitor1, mockExecutor1, mockChecker1, _, apiServer1, store1 := prepareDirect(t, false)
	mockMonitor2, mockExecutor2, mockChecker2, _, apiServer2, store2 := prepareDirect(t, false)
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

	time.Sleep(10 * time.Second)
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
