package router

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	from  = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b991"
	to    = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b992"
	other = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b993"
)

func TestUnionRouter_Route(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockPeerManager := mock_peermgr.NewMockPeerManager(mockCtl)

	message := peermgr.Message(peerproto.Message_ACK, true, nil)
	mockPeerManager.EXPECT().FindProviders(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Connect(gomock.Any()).Return(other, nil).AnyTimes()
	mockPeerManager.EXPECT().Send(gomock.Any(), gomock.Any()).Return(message, nil).AnyTimes()

	repoRoot, err := ioutil.TempDir("", "router_commit")
	assert.Nil(t, err)
	storage, err := leveldb.New(filepath.Join(repoRoot, "storage"))

	router := New(mockPeerManager, storage, log.NewWithModule("router"))

	router.Start()

	ibtp := mockIBTP(t, 1, pb.IBTP_INTERCHAIN)
	err = router.Route(ibtp)
	require.Nil(t, err)

	// send repeated ibtp
	err = router.Route(ibtp)
	require.Nil(t, err)

	ibtp.From = ibtp.To
	err = router.Route(ibtp)
	require.Nil(t, err)

	router.Stop()


}

func TestUnionRouter_AddAppchains(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockPeerManager := mock_peermgr.NewMockPeerManager(mockCtl)
	mockPeerManager.EXPECT().Provider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	repoRoot, err := ioutil.TempDir("", "router_commit")
	assert.Nil(t, err)
	storage, err := leveldb.New(filepath.Join(repoRoot, "storage"))

	router := New(mockPeerManager, storage, log.NewWithModule("router"))

	appchains := make([]*rpcx.Appchain, 0)
	app := &rpcx.Appchain{
		ID:   from,
		Name: "app",
	}
	bxh := &rpcx.Appchain{
		ID:   to,
		Name: "bxh",
	}
	appchains = append(appchains, app, bxh)
	err = router.AddAppchains(appchains)
	require.Nil(t, err)

	ok := router.ExistAppchain(app.ID)
	require.True(t, ok)

	ok = router.ExistAppchain(bxh.ID)
	require.True(t, ok)
}

func mockIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	content := pb.Content{
		SrcContractId: from,
		DstContractId: from,
		Func:          "set",
	}

	bytes, err := content.Marshal()
	assert.Nil(t, err)

	ibtppd, err := json.Marshal(pb.Payload{
		Encrypted: false,
		Content:   bytes,
	})
	assert.Nil(t, err)

	return &pb.IBTP{
		From:      from,
		To:        to,
		Payload:   ibtppd,
		Index:     index,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
	}
}
