package appchain

import (
	"encoding/json"

	"github.com/meshplus/pier/internal/peermgr"

	"github.com/sirupsen/logrus"

	"github.com/libp2p/go-libp2p-core/network"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
)

func (mgr *Manager) handleMessage(s network.Stream, msg *peerproto.Message) {
	app := appchainmgr.Appchain{}
	if err := json.Unmarshal(msg.Payload.Data, &app); err != nil {
		logger.Error(err)
		return
	}

	var res []byte
	var ok bool
	switch msg.Type {
	case peerproto.Message_APPCHAIN_REGISTER:
		ok, res = mgr.Mgr.Register(app.ID, app.Validators, app.ConsensusType,
			app.ChainType, app.Name, app.Desc, app.Version, app.PublicKey)
	case peerproto.Message_APPCHAIN_UPDATE:
		ok, res = mgr.Mgr.UpdateAppchain(app.ID, app.Validators, app.ConsensusType,
			app.ChainType, app.Name, app.Desc, app.Version, app.PublicKey)
	case peerproto.Message_APPCHAIN_GET:
		ok, res = mgr.Mgr.GetAppchain(app.ID)
	default:
		m := "wrong appchain message type"
		res = []byte(m)
		logger.Error(m)
	}

	ackMsg := peermgr.Message(peerproto.Message_ACK, ok, res)
	err := mgr.PeerManager.SendWithStream(s, ackMsg)
	if err != nil {
		logger.Error(err)
	}

	logger.WithFields(logrus.Fields{
		"type":           msg.Type,
		"from_id":        app.ID,
		"name":           app.Name,
		"desc":           app.Desc,
		"chain_type":     app.ChainType,
		"consensus_type": app.ConsensusType,
	}).Info("Handle appchain message")
}
