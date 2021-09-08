package appchain

import (
	"encoding/json"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	pb "github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

func (mgr *Manager) handleMessage(s network.Stream, msg *pb.Message) {
	var res []byte
	var ok bool
	switch msg.Type {
	case pb.Message_APPCHAIN_REGISTER:
		// todo check err
		//ok, res = mgr.Mgr.Register(msg.Payload.Data)
	case pb.Message_APPCHAIN_UPDATE:
		//ok, res = mgr.Mgr.Update(msg.Payload.Data)
	case pb.Message_APPCHAIN_GET:
		app := &appchainmgr.Appchain{}
		if err := json.Unmarshal(peermgr.DataToPayload(msg).Data, app); err != nil {
			mgr.logger.Error(err)
			return
		}
		//res, _ = mgr.Mgr.QueryById(app.ID, nil)
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

	// todo check err
	mgr.logger.WithFields(logrus.Fields{
		"type":    msg.Type,
		"from_id": appchainRes.ID,
		//"name":           appchainRes.Name,
		"desc": appchainRes.Desc,
		//"chain_type":     appchainRes.ChainType,
		//"consensus_type": appchainRes.ConsensusType,
	}).Info("Handle appchain message")
}
