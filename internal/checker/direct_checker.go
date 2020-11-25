package checker

import (
	"encoding/json"
	"fmt"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/rulemgr"
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
	chainID := ibtp.From
	if ibtp.Type == pb.IBTP_RECEIPT_SUCCESS || ibtp.Type == pb.IBTP_RECEIPT_FAILURE {
		chainID = ibtp.To
	}

	ok, appchainByte := c.appchainMgr.Mgr.GetAppchain(chainID)
	if !ok {
		return fmt.Errorf("appchain %s not found", chainID)
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
