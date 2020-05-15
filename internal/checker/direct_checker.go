package checker

import (
	"encoding/json"
	"fmt"

	"github.com/meshplus/pier/internal/rulemgr"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/appchain"
)

type DirectChecker struct {
	ruleMgr     *rulemgr.RuleMgr
	appchainMgr *appchain.Manager
}

func NewDirectChecker(ruleMgr *rulemgr.RuleMgr, appchainMgr *appchain.Manager) Checker {
	return &DirectChecker{
		ruleMgr:     ruleMgr,
		appchainMgr: appchainMgr,
	}
}

func (c *DirectChecker) Check(ibtp *pb.IBTP) error {
	ok, appchainByte := c.appchainMgr.Mgr.GetAppchain(ibtp.From)
	if !ok {
		return fmt.Errorf("appchain not found")
	}

	appchain := &appchainmgr.Appchain{}
	if err := json.Unmarshal(appchainByte, appchain); err != nil {
		return fmt.Errorf("unmarshal appchain: %w", err)
	}

	ok, err := c.ruleMgr.Validate(ibtp.From, ibtp.From, ibtp.Proof, ibtp.Payload, appchain.Validators)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("rule check failed")
	}

	return nil
}
