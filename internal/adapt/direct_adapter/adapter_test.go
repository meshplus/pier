package direct_adapter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/adapt/mock_adapt"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	fromHash   = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	toHash     = "0xabc2s384524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db60e"
	appChainId = "testAppChain"
)

func TestID(t *testing.T) {
	adapter, _, _, _ := prepare(t)
	id := adapter.ID()
	require.Equal(t, appChainId, id)
}

func TestName(t *testing.T) {
	adapter1, _, _, _ := prepare(t)
	name := adapter1.Name()
	require.Equal(t, fmt.Sprintf("direct:%s", appChainId), name)
}

func TestStart(t *testing.T) {
	adapter, _, peerMgr, _ := prepare(t)

	// start normal
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(3)
	peerMgr.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	peerMgr.EXPECT().Start().Return(nil).Times(1)
	peerMgr.EXPECT().CountConnectedPeers().Return(uint64(1)).Times(1)
	peers := make(map[string]*peer.AddrInfo, 1)
	peers[appChainId] = &peer.AddrInfo{}
	peerMgr.EXPECT().Peers().Return(peers).AnyTimes()

	peerMgr.EXPECT().Stop().Return(nil).Times(1)
	err := adapter.Start()
	require.Nil(t, err)
	err = adapter.Stop()
	require.Nil(t, err)

	// test register interchain error
	interchainErr := fmt.Errorf("register query interchain msg handler")
	peerMgr.EXPECT().RegisterMsgHandler(pb.Message_INTERCHAIN_META_GET, gomock.Any()).Return(interchainErr).Times(1)
	err = adapter.Start()
	require.Equal(t, true, strings.Contains(err.Error(), interchainErr.Error()))

	// test peerMgr start error
	startError := fmt.Errorf("start peerMgr err")
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(3)
	peerMgr.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)
	peerMgr.EXPECT().Start().Return(startError).MaxTimes(1)
	err = adapter.Start()
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), startError.Error()))

	// test connceted peer num error
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(3)
	peerMgr.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)
	peerMgr.EXPECT().Start().Return(nil).MaxTimes(1)
	peerMgr.EXPECT().CountConnectedPeers().Return(uint64(2)).MaxTimes(1)
	err = adapter.Start()
	require.NotNil(t, err)

	// test peerMgr stop err
	stopError := fmt.Errorf("stop peerMgr err")
	peerMgr.EXPECT().Stop().Return(stopError).AnyTimes()
	err = adapter.Stop()
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), stopError.Error()))

}

func TestSendIBTP(t *testing.T) {
	adapter1, adapter2, peerMgr, _ := prepare(t)

	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	receipt1 := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	ibtp2 := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	ibtp4 := getIBTP(t, 4, pb.IBTP_INTERCHAIN)

	fromPierId := "testappchain1"
	toPierId := "testappchain2"
	peerMgr.EXPECT().AsyncSend(gomock.Any(), gomock.Any()).Do(func(pierId string, msg *pb.Message) {
		var stream network.Stream
		if pierId == fromPierId {
			adapter1.handleSendIBTPMessage(stream, msg)
		}
		if pierId == toPierId {
			adapter2.handleSendIBTPMessage(stream, msg)
		}
	}).Return(nil).MaxTimes(3)

	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	peerMgr.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	peerMgr.EXPECT().Start().Return(nil).AnyTimes()
	peerMgr.EXPECT().CountConnectedPeers().Return(uint64(1)).AnyTimes()
	peers1 := make(map[string]*peer.AddrInfo, 1)
	peers1[toPierId] = &peer.AddrInfo{}
	peerMgr.EXPECT().Peers().Return(peers1).MaxTimes(1)
	err := adapter1.Start()
	require.Nil(t, err)

	peers2 := make(map[string]*peer.AddrInfo, 1)
	peers2[fromPierId] = &peer.AddrInfo{}
	peerMgr.EXPECT().Peers().Return(peers2).MaxTimes(1)
	err = adapter2.Start()
	require.Nil(t, err)

	err = adapter1.SendIBTP(ibtp1)
	require.Nil(t, err)
	err = adapter1.SendIBTP(ibtp2)
	require.Nil(t, err)

	err = adapter2.SendIBTP(receipt1)
	require.Nil(t, err)

	time.Sleep(2 * time.Second)
	revCh2 := adapter2.MonitorIBTP()
	require.Equal(t, 2, len(revCh2)+adapter2.ibtpCache.Len())

	//todo： why not alway equal 4 ?
	require.Equal(t, uint64(2), adapter2.maxIndexMap[toHash])
	require.Equal(t, uint64(1), adapter1.maxIndexMap[toHash])

	wrongTypeIbtp := getIBTP(t, 1, pb.IBTP_RECEIPT_ROLLBACK)
	err = adapter1.SendIBTP(wrongTypeIbtp)
	require.Equal(t, true, strings.Contains(err.Error(), "unsupport ibtp type"))

	// test AsyncSend err
	asyncSendErr := fmt.Errorf("send ibtp to pier err")
	peerMgr.EXPECT().AsyncSend(gomock.Any(), gomock.Any()).Return(asyncSendErr).MaxTimes(1)
	err = adapter1.SendIBTP(ibtp4)
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), asyncSendErr.Error()))

	peerMgr.EXPECT().Stop().Return(nil).AnyTimes()
	err = adapter1.Stop()
	require.Nil(t, err)
	err = adapter2.Stop()
	require.Nil(t, err)

}

func TestQueryIBTP(t *testing.T) {
	adapter1, _, peerMgr, _ := prepare(t)

	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	id1 := ibtp1.ID()
	data, err := ibtp1.Marshal()
	require.Nil(t, err)
	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	peerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(retMsg, nil).MaxTimes(1)

	ret1, err := adapter1.QueryIBTP(id1, true)
	require.Nil(t, err)
	require.Equal(t, ibtp1, ret1)

	// test wrong ibtpId and serviceId
	wrongId1 := "wrongIBTPId"
	_, err = adapter1.QueryIBTP(wrongId1, true)
	require.Equal(t, true, strings.Contains(err.Error(), wrongId1))

	// test qurey ibtp from ibtpCache firstly, If there is no ibtp in the cache, connect the network
	ibtp2 := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	id2 := ibtp2.ID()
	adapter1.ibtpCache.Add(ibtp2.Index, ibtp2)
	ret2, err := adapter1.QueryIBTP(id2, true)
	require.Nil(t, err)
	require.Equal(t, ibtp2, ret2)

	// test get wrong type from ibtpCache
	wrongType := &pb.Message{}
	wrongId3 := fmt.Sprintf("%s-%s-%d", "from", "to", 0)
	adapter1.ibtpCache.Add(uint64(0), wrongType)
	ret3, err := adapter1.QueryIBTP(wrongId3, true)
	require.Nil(t, ret3)
	queryError := fmt.Errorf("get wrong type from ibtpCache")
	require.Equal(t, err, queryError)

	receipt1 := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	receiptId := receipt1.ID()
	dataReceipt1, err := receipt1.Marshal()
	require.Nil(t, err)
	receiptMsg := peermgr.Message(pb.Message_ACK, true, dataReceipt1)
	peerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(receiptMsg, nil).MaxTimes(1)

	retReceipt, err := adapter1.QueryIBTP(receiptId, false)
	require.Nil(t, err)
	require.Equal(t, receipt1, retReceipt)

}

func TestQueryInterchain(t *testing.T) {
	adapter1, _, peerMgr, _ := prepare(t)
	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	id1 := ibtp1.GetFrom()
	interchain := &pb.Interchain{
		InterchainCounter:       map[string]uint64{},
		ReceiptCounter:          map[string]uint64{},
		SourceInterchainCounter: map[string]uint64{},
		SourceReceiptCounter:    map[string]uint64{},
	}
	data, err := interchain.Marshal()
	require.Nil(t, err)
	retMsg := peermgr.Message(pb.Message_ACK, true, data)
	peerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(retMsg, nil)
	ret1, err := adapter1.QueryInterchain(id1)
	require.Nil(t, err)
	require.Equal(t, interchain, ret1)

}

// todo: mock swarm, refactor the unit test in the next version
func TestHandleGetIBTPMessage(t *testing.T) {
	adapter, _, peerMgr, appchainAdapt := prepare(t)
	var stream network.Stream

	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	id := ibtp1.ID()
	appchainAdapt.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(ibtp1, nil).AnyTimes()
	peerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil)
	msg := peermgr.Message(pb.Message_IBTP_GET, true, []byte(id))
	adapter.handleGetIBTPMessage(stream, msg)
}

func TestHandleGetInterchainMessage(t *testing.T) {
	adapter, _, peerMgr, appchainAdapt := prepare(t)
	var stream network.Stream
	serviceID := fmt.Sprintf(":%s:%s", appChainId, toHash)
	msg := peermgr.Message(pb.Message_INTERCHAIN_META_GET, true, []byte(serviceID))
	peerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil)
	appchainAdapt.EXPECT().QueryInterchain(serviceID).Return(&pb.Interchain{
		InterchainCounter:       map[string]uint64{},
		ReceiptCounter:          map[string]uint64{},
		SourceInterchainCounter: map[string]uint64{},
		SourceReceiptCounter:    map[string]uint64{},
	}, nil)
	adapter.handleGetInterchainMessage(stream, msg)
}

func prepare(t *testing.T) (*DirectAdapter, *DirectAdapter, *mock_peermgr.MockPeerManager, *mock_adapt.MockAdapt) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()

	config := &repo.Config{}
	config.Mode.Type = repo.DirectMode
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)
	mockAppChainAdapt := mock_adapt.NewMockAdapt(mockCtl)
	mockAppChainAdapt.EXPECT().ID().Return(appChainId).AnyTimes()
	adapter1, err := New(mockPeerMgr, mockAppChainAdapt, log.NewWithModule("direct_adapter1"))
	adapter2, err := New(mockPeerMgr, mockAppChainAdapt, log.NewWithModule("direct_adapter2"))
	require.Nil(t, err)

	return adapter1, adapter2, mockPeerMgr, mockAppChainAdapt
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	ct := &pb.Content{
		Func: "set",
		Args: [][]byte{[]byte("Alice")},
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
		From:    fmt.Sprintf(":testappchain1:%s", fromHash),
		To:      fmt.Sprintf(":testappchain2:%s", toHash),
		Payload: ibtppd,
		Index:   index,
		Type:    typ,
	}
}
