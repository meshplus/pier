package router

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
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
