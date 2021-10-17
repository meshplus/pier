package union_adapter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/adapt/mock_adapt"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	fromHash = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	toHash   = "0xabc2s384524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db60e"
	bxhId    = "1356"
	other    = "1358:chain2:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b993"
)

func TestName(t *testing.T) {
	adapter1, _, _, _ := prepare(t)
	name := adapter1.Name()
	require.Equal(t, fmt.Sprintf("union:%s", bxhId), name)
}

func TestStart(t *testing.T) {
	adapter, _, peerMgr, _ := prepare(t)

	// start normal
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(4)
	peerMgr.EXPECT().Start().Return(nil).Times(1)
	peerMgr.EXPECT().Stop().Return(nil).Times(1)
	err := adapter.Start()
	require.Nil(t, err)
	err = adapter.Stop()
	require.Nil(t, err)

	// test register interchain error
	interchainErr := fmt.Errorf("register query interchain msg handler")
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Eq(pb.Message_ROUTER_INTERCHAIN_GET), gomock.Any()).Return(interchainErr).Times(1)
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Not(pb.Message_ROUTER_INTERCHAIN_GET), gomock.Any()).Return(nil).Times(3)
	err = adapter.Start()
	require.Equal(t, true, strings.Contains(err.Error(), interchainErr.Error()))

	// test register interchain error
	interchainErr2 := fmt.Errorf("register router ibtp receipt get handler")
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Eq(pb.Message_ROUTER_IBTP_RECEIPT_GET), gomock.Any()).Return(interchainErr).Times(1)
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Not(pb.Message_ROUTER_IBTP_RECEIPT_GET), gomock.Any()).Return(nil).Times(3)
	err = adapter.Start()
	require.Equal(t, true, strings.Contains(err.Error(), interchainErr2.Error()))

	// test peerMgr start error
	startError := fmt.Errorf("start peerMgr err")
	peerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(4)
	peerMgr.EXPECT().Start().Return(startError).Times(1)
	err = adapter.Start()
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), startError.Error()))

	// test peerMgr stop err
	stopError := fmt.Errorf("stop peerMgr err")
	peerMgr.EXPECT().Stop().Return(stopError).AnyTimes()
	err = adapter.Stop()
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), stopError.Error()))

}

func TestSendIBTP(t *testing.T) {
	adapter1, _, peerMgr, _ := prepare(t)
	message := peermgr.Message(pb.Message_ACK, true, nil)
	peerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(message, nil).AnyTimes()
	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	receipt1 := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
	ibtp2 := getIBTP(t, 2, pb.IBTP_INTERCHAIN)
	ibtp4 := getIBTP(t, 4, pb.IBTP_INTERCHAIN)

	err := adapter1.SendIBTP(ibtp1)
	require.Nil(t, err)
	err = adapter1.SendIBTP(receipt1)
	require.Nil(t, err)
	err = adapter1.SendIBTP(ibtp2)
	require.Nil(t, err)
	err = adapter1.SendIBTP(ibtp4)
	require.Nil(t, err)
	wrongIdIbtp := &pb.IBTP{
		From:    fmt.Sprintf(":testappchain1:%s", fromHash),
		To:      fmt.Sprintf("wrongType:%s", toHash),
		Payload: nil,
		Index:   5,
		Type:    pb.IBTP_INTERCHAIN,
	}
	err = adapter1.SendIBTP(wrongIdIbtp)
	require.Equal(t, true, strings.Contains(err.Error(), "invalid service ID"))
	time.Sleep(1 * time.Second)

	peerMgr.EXPECT().Stop().Return(nil).AnyTimes()

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
	wrongId2 := fmt.Sprintf("%s-%s-%d", "wrongServiceId", "wrongServiceId", 1)
	_, err = adapter1.QueryIBTP(wrongId2, true)
	require.Equal(t, true, strings.Contains(err.Error(), "wrongServiceId"))
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

	// test wrong service id
	wrongServiceId := "wrongServiceId"
	ret2, err := adapter1.QueryInterchain(wrongServiceId)
	require.Nil(t, ret2)
	require.Equal(t, true, strings.Contains(err.Error(), "wrongServiceId"))

}

func TestHandleGetIBTPMessage(t *testing.T) {
	adapter, _, peerMgr, appchainAdapt := prepare(t)
	var stream network.Stream

	ibtp1 := getIBTP(t, 1, pb.IBTP_INTERCHAIN)
	id := ibtp1.ID()
	appchainAdapt.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(ibtp1, nil).AnyTimes()
	peerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil)
	msg := peermgr.Message(pb.Message_IBTP_GET, true, []byte(id))
	adapter.handleRouterGetIBTPMessage(stream, msg)

}

func prepare(t *testing.T) (*UnionAdapter, *UnionAdapter, *mock_peermgr.MockPeerManager, *mock_adapt.MockAdapt) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	loggers.InitializeLogger(repo.DefaultConfig())
	config := &repo.Config{}
	config.Mode.Type = repo.UnionMode
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)
	mockBxhChainAdapt := mock_adapt.NewMockAdapt(mockCtl)
	mockBxhChainAdapt.EXPECT().ID().Return(bxhId).AnyTimes()
	mockPeerMgr.EXPECT().FindProviders(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerMgr.EXPECT().Connect(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerMgr.EXPECT().ConnectedPeerIDs().Return([]string{""}).AnyTimes()
	mockPeerMgr.EXPECT().Provider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter1, err := New(mockPeerMgr, mockBxhChainAdapt, log.NewWithModule("union_adapter1"))
	adapter2, err := New(mockPeerMgr, mockBxhChainAdapt, log.NewWithModule("union_adapter2"))
	require.Nil(t, err)

	return adapter1, adapter2, mockPeerMgr, mockBxhChainAdapt
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
