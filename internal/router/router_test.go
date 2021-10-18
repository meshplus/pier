package router

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	from  = "1356:chain0:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b991"
	to    = "1357:chain1:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b992"
	other = "1358:chain2:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b993"
)

func TestUnionRouter_Route(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockPeerManager := mock_peermgr.NewMockPeerManager(mockCtl)

	message := peermgr.Message(pb.Message_ACK, true, nil)
	mockPeerManager.EXPECT().FindProviders(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Connect(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Send(gomock.Any(), gomock.Any()).Return(message, nil).AnyTimes()
	mockPeerManager.EXPECT().ConnectedPeerIDs().Return([]string{""}).AnyTimes()
	mockPeerManager.EXPECT().Provider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	//repoRoot, err := ioutil.TempDir("", "router_commit")
	//assert.Nil(t, err)
	//storage, err := leveldb.New(filepath.Join(repoRoot, "storage"))

	router := New(mockPeerManager, log.NewWithModule("router"))
	router.Start()

	ibtp := mockIBTP(t, 1, pb.IBTP_INTERCHAIN)
	err := router.Route(ibtp)
	require.Nil(t, err)

	// send repeated ibtp
	err = router.Route(ibtp)
	require.Nil(t, err)

	ibtp.From = ibtp.To
	err = router.Route(ibtp)
	require.Nil(t, err)

	err = router.Broadcast("1356")
	require.Nil(t, err)

	router.Stop()

}

func TestUnionRouter_Interchain(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockPeerManager := mock_peermgr.NewMockPeerManager(mockCtl)
	mockPeerManager.EXPECT().FindProviders(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Connect(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().ConnectedPeerIDs().Return([]string{""}).AnyTimes()
	router := New(mockPeerManager, log.NewWithModule("router"))
	router.Start()

	// test query interchain
	message := peermgr.Message(pb.Message_ROUTER_INTERCHAIN_GET, true, []byte(to))
	interchainBegin := &pb.Interchain{ID: from,
		SourceInterchainCounter: map[string]uint64{to: 1},
		ReceiptCounter:          map[string]uint64{to: 1},
		SourceReceiptCounter:    map[string]uint64{to: 1},
		InterchainCounter:       map[string]uint64{to: 1}}
	interchainBeginMarshal, _ := interchainBegin.Marshal()
	returnMessage := peermgr.Message(pb.Message_ACK, true, interchainBeginMarshal)
	mockPeerManager.EXPECT().Send(other, message).Return(returnMessage, nil).AnyTimes()

	interchain, err := router.QueryInterchain("1356", to)
	require.Nil(t, err)
	marshal, _ := interchain.Marshal()
	require.Equal(t, marshal, interchainBeginMarshal)

	interchain2, err := router.QueryInterchain("1356", to)
	require.Nil(t, err)
	marshal2, _ := interchain2.Marshal()
	require.Equal(t, marshal2, interchainBeginMarshal)
}

func TestUnionRouter_IBTP(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockPeerManager := mock_peermgr.NewMockPeerManager(mockCtl)
	mockPeerManager.EXPECT().FindProviders(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Connect(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().ConnectedPeerIDs().Return([]string{""}).AnyTimes()
	router := New(mockPeerManager, log.NewWithModule("router"))
	router.Start()

	// test query interchain
	message := peermgr.Message(pb.Message_ROUTER_IBTP_GET, true, []byte(from+"-"+to+"-1"))
	ibtpBegin := &pb.IBTP{
		From:    from,
		To:      to,
		Index:   1,
		Payload: []byte{1},
	}
	ibtpBeginMarshal, _ := ibtpBegin.Marshal()
	returnMessage := peermgr.Message(pb.Message_ACK, true, ibtpBeginMarshal)
	mockPeerManager.EXPECT().Send(other, message).Return(returnMessage, nil).AnyTimes()

	ibtp, err := router.QueryIBTP(from+"-"+to+"-1", true)
	require.Nil(t, err)
	marshal, _ := ibtp.Marshal()
	require.Equal(t, marshal, ibtpBeginMarshal)

	ibtp2, err := router.QueryIBTP(from+"-"+to+"-1", true)
	require.Nil(t, err)
	marshal2, _ := ibtp2.Marshal()
	require.Equal(t, marshal2, ibtpBeginMarshal)
}

func mockIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	content := pb.Content{
		Func: "set",
	}

	bytes, err := content.Marshal()
	assert.Nil(t, err)

	ibtppd, err := json.Marshal(pb.Payload{
		Encrypted: false,
		Content:   bytes,
	})
	assert.Nil(t, err)

	return &pb.IBTP{
		From:    from,
		To:      to,
		Payload: ibtppd,
		Index:   index,
		Type:    typ,
	}
}
