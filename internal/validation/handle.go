package validation

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/meshplus/bitxhub-kit/types"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
)

func (rm *RuleMgr) handleRule(net network.Stream, msg *peerproto.Message) {
	data := msg.Data
	rule := &Rule{}
	if err := json.Unmarshal(data, rule); err != nil {
		logger.Error(err)
		return
	}
	ok := true
	err := rm.Ledger.SetCode(types.String2Address(rule.Address), rule.Code)
	if err != nil {
		ok = false
		logger.Error(err)
	}

	res := RuleResponse{
		Ok:      ok,
		Content: err.Error(),
	}

	ackData, err := json.Marshal(res)
	if err != nil {
		logger.Error(err)
	}

	ackMsg := peerproto.Message{
		Type: peerproto.Message_RULE_ACK,
		Data: ackData,
	}

	err = rm.PeerManager.SendWithStream(net, &ackMsg)
	if err != nil {
		logger.Error(err)
	}
}
