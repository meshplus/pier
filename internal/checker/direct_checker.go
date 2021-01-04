package checker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-core/validator"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/rulemgr"
)

type DirectChecker struct {
	ruleMgr       *rulemgr.RuleMgr
	appchainMgr   *appchain.Manager
	appchainCache sync.Map
}

type appchainRule struct {
	appchain    *appchainmgr.Appchain
	codeAddress string
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

	appchainLoad, ok := c.appchainCache.Load(chainID)
	var appchain *appchainmgr.Appchain
	var validatorAddr string
	if !ok {
		ok, appchainByte := c.appchainMgr.Mgr.GetAppchain(chainID)
		if !ok {
			return fmt.Errorf("appchain %s not found", chainID)
		}

		appchain = &appchainmgr.Appchain{}
		if err := json.Unmarshal(appchainByte, appchain); err != nil {
			return fmt.Errorf("unmarshal appchain: %w", err)
		}

		chainAddr := types.NewAddressByStr(chainID)
		code := c.ruleMgr.Ledger.GetCode(*chainAddr)
		if code == nil {
			if appchain.ChainType == "fabric" {
				validatorAddr = validator.SimFabricRuleAddr
			} else {
				return fmt.Errorf("not found rule address from appchain:%s", appchain.ID)
			}
		} else {
			codeHash := sha256.Sum256(code)
			validatorAddr = types.NewAddress(codeHash[:]).String()
		}

		c.appchainCache.Store(chainID, &appchainRule{
			appchain:    appchain,
			codeAddress: validatorAddr,
		})
	} else {
		appchainRule := appchainLoad.(*appchainRule)
		appchain = appchainRule.appchain
		validatorAddr = appchainRule.codeAddress
	}

	ok, err := c.ruleMgr.Validate(validatorAddr, ibtp.From, ibtp.Proof, ibtp.Payload, appchain.Validators)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("rule check failed")
	}

	return nil
}
