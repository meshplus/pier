package router

import (
	"encoding/json"
	"github.com/golang/mock/gomock"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"testing"
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

	router := New(mockPeerManager, storage, log.NewWithModule("router"), []string{""})
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

	router := New(mockPeerManager, storage, log.NewWithModule("router"), []string{""})
	appchains := make([]*appchainmgr.Appchain, 0)
	app := &appchainmgr.Appchain{
		ID:   from,
		Name: "app",
	}
	bxh := &appchainmgr.Appchain{
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
