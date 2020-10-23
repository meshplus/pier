package appchain

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/stretchr/testify/require"
)

const (
	appchainId = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b991"
	pierId     = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestRegisterAppchain(t *testing.T) {
	manager := prepare(t)
	appchain := appchain()
	var s network.Stream
	data, err := json.Marshal(appchain)
	require.Nil(t, err)

	msg := peermgr.Message(peerproto.Message_APPCHAIN_REGISTER, true, data)
	manager.handleMessage(s, msg)
	require.Nil(t, err)

	ok, count := manager.Mgr.CountAppchains()
	require.Equal(t, ok, true)
	require.Equal(t, string(count), "1")
}

func TestUpdateAppchain(t *testing.T) {
	manager := prepare(t)
	appchain := appchain()

	data, err := json.Marshal(appchain)
	require.Nil(t, err)

	var s network.Stream
	msg := peermgr.Message(peerproto.Message_APPCHAIN_REGISTER, true, data)
	manager.handleMessage(s, msg)
	require.Nil(t, err)

	appchain.Name = "fabric"
	data1, err := json.Marshal(appchain)
	require.Nil(t, err)

	manager.Mgr.Audit(appchainId, 1, "")

	msg1 := peermgr.Message(peerproto.Message_APPCHAIN_UPDATE, true, data1)
	manager.handleMessage(s, msg1)

	ok, res := manager.Mgr.GetAppchain(appchainId)
	require.Equal(t, ok, true)

	var resAppchain appchainmgr.Appchain
	err = json.Unmarshal(res, &resAppchain)
	require.Nil(t, err)
	require.Equal(t, resAppchain.Name, "fabric")
}

func prepare(t *testing.T) *Manager {
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	store, err := leveldb.New(tmpDir)
	require.Nil(t, err)

	mockCtl := gomock.NewController(t)
	mockPeerMgr := mock_peermgr.NewMockPeerManager(mockCtl)
	mockPeerMgr.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	manager, err := NewManager(pierId, store, mockPeerMgr)
	require.Nil(t, err)
	return manager
}

func appchain() *appchainmgr.Appchain {
	appchain := &appchainmgr.Appchain{
		ID:            appchainId,
		Name:          "appchainA",
		Validators:    "",
		ConsensusType: 0,
		Status:        0,
		ChainType:     "1",
		Desc:          "appchain",
		Version:       "1.0.0",
		PublicKey:     "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b99",
	}
	return appchain
}
