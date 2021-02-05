package appchain

import (
	"encoding/json"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/sirupsen/logrus"
)

func (mgr *Manager) handleMessage(s network.Stream, msg *peerproto.Message) {
	app := appchainmgr.Appchain{}
	if err := json.Unmarshal(msg.Payload.Data, &app); err != nil {
		mgr.logger.Error(err)
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
		mgr.logger.Error(m)
	}

	ackMsg := peermgr.Message(msg.Type, ok, res)
	err := mgr.PeerManager.AsyncSendWithStream(s, ackMsg)
	if err != nil {
		mgr.logger.Error(err)
	}

	appchainRes := &appchainmgr.Appchain{}
	if err := json.Unmarshal(res, appchainRes); err != nil {
		mgr.logger.Error(err)
		return
	}

	mgr.logger.WithFields(logrus.Fields{
		"type":           msg.Type,
		"from_id":        appchainRes.ID,
		"name":           appchainRes.Name,
		"desc":           appchainRes.Desc,
		"chain_type":     appchainRes.ChainType,
		"consensus_type": appchainRes.ConsensusType,
	}).Info("Handle appchain message")
}
