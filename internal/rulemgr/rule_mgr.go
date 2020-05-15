package rulemgr

import (
	"github.com/meshplus/bitxhub-core/validator"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
)

var logger = log.NewWithModule("rule_mgr")

type Rule struct {
	Code    []byte `json:"code"`
	Address string `json:"address"`
}

type RuleResponse struct {
	Ok      bool   `json:"ok"`
	Content string `json:"content"`
}

type RuleMgr struct {
	Ledger      *CodeLedger
	PeerManager peermgr.PeerManager
	Ve          *validator.ValidationEngine
}

func New(storage storage.Storage, pm peermgr.PeerManager) (*RuleMgr, error) {
	ledger := &CodeLedger{
		storage: storage,
	}
	ve := validator.NewValidationEngine(ledger, logger)
	rm := &RuleMgr{
		Ledger:      ledger,
		PeerManager: pm,
		Ve:          ve,
	}
	if err := pm.RegisterMsgHandler(peerproto.Message_RULE_DEPLOY, rm.handleRule); err != nil {
		return nil, err
	}
	return rm, nil
}

func (rm *RuleMgr) Validate(address, from string, proof, payload []byte, validators string) (bool, error) {
	return rm.Ve.Validate(address, from, proof, payload, validators)
}
