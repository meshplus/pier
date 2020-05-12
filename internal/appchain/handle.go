package appchain

import (
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p-core/network"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/pier/internal/appchain/proto"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
)

func (mgr *AppchainMgr) handleAppchain(net network.Stream, msg *peerproto.Message) {
	data := msg.Data
	am := &proto.AppchainMessage{}
	if err := am.Unmarshal(data); err != nil {
		logger.Error(err)
		return
	}
	app := appchainmgr.Appchain{}
	if err := json.Unmarshal(am.Data, &app); err != nil {
		logger.Error(err)
		return
	}
	var ok bool
	var res []byte
	switch am.Type {
	case proto.AppchainMessage_REGISTER:
		ok, res = mgr.Mgr.Register(app.ID, app.Validators, app.ConsensusType,
			app.ChainType, app.Name, app.Desc, app.Version, app.PublicKey)
	case proto.AppchainMessage_UPDATE:
		ok, res = mgr.Mgr.UpdateAppchain(app.ID, app.Validators, app.ConsensusType,
			app.ChainType, app.Name, app.Desc, app.Version, app.PublicKey)
	case proto.AppchainMessage_GET:
		ok, res = mgr.Mgr.GetAppchain(app.ID)
	default:
		logger.Error(fmt.Sprintf("unexepect proto's type of appchain"))
		return
	}

	if !ok {
		logger.Error(string(res))
	}

	ackAm := proto.AppchainMessage{
		Type: am.Type,
		Ok:   ok,
		Data: res,
	}
	ackAmData, err := ackAm.Marshal()
	if err != nil {
		logger.Error(string(res))
		return
	}

	ackMsg := peerproto.Message{
		Type: peerproto.Message_APPCHAIN_ACK,
		Data: ackAmData,
	}

	err = mgr.PeerManager.SendWithStream(net, &ackMsg)
	if err != nil {
		logger.Error(err)
	}

}
