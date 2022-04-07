package rulemgr

import (
	"encoding/json"

	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/peermgr"
)

func (rm *RuleMgr) handleRule(net network.Stream, msg *pb.Message) {
	data := peermgr.DataToPayload(msg).Data
	rule := &Rule{}
	if err := json.Unmarshal(data, rule); err != nil {
		rm.logger.Error(err)
		return
	}
	ok := true
	err := rm.Ledger.SetCode(types.NewAddressByStr(rule.Address), rule.Code)
	if err != nil {
		ok = false
		rm.logger.Error(err)
	}

	res := RuleResponse{
		Ok: ok,
	}

	ackData, err := json.Marshal(res)
	if err != nil {
		rm.logger.Error(err)
		return
	}

	ackMsg := peermgr.Message(pb.Message_ACK, true, ackData)

	err = rm.PeerManager.AsyncSendWithStream(net, ackMsg)
	if err != nil {
		rm.logger.Error(err)
	}
}
