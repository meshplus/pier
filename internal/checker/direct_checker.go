package checker

import (
	"encoding/json"
	"fmt"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/validation"
)

type DirectChecker struct {
	ruleMgr     *validation.RuleMgr
	appchainMgr *appchain.AppchainMgr
}

func NewDirectChecker(ruleMgr *validation.RuleMgr, appchainMgr *appchain.AppchainMgr) Checker {
	return &DirectChecker{
		ruleMgr:     ruleMgr,
		appchainMgr: appchainMgr,
	}
}

func (c *DirectChecker) Check(ibtp *pb.IBTP) (bool, error) {
	ok, appchainByte := c.appchainMgr.Mgr.GetAppchain(ibtp.From)
	if !ok {
		return false, fmt.Errorf("appchain not found")
	}
	appchain := &appchainmgr.Appchain{}
	if err := json.Unmarshal(appchainByte, appchain); err != nil {
		return false, err
	}

	return c.ruleMgr.Ve.Validate(ibtp.From, ibtp.From, ibtp.Proof, ibtp.Payload, appchain.Validators)
}
