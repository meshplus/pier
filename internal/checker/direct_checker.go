package checker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-core/validator"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/sirupsen/logrus"
)

var _ Checker = (*DirectChecker)(nil)

type DirectChecker struct {
	ve         *validator.ValidationEngine
	client     agency.Client
	chainInfoM map[string]*AppchainInfo
	appchainID string
}

type AppchainInfo struct {
	broker    string
	trustRoot []byte
	ruleAddr  string
}

func NewDirectChecker(client agency.Client, appchainID string, logger logrus.FieldLogger, gasLimit uint64) Checker {
	ledger := &CodeLedger{
		validateCode: make(map[string][]byte),
	}

	return &DirectChecker{
		ve:         validator.NewValidationEngine(ledger, &sync.Map{}, logger, gasLimit),
		client:     client,
		chainInfoM: make(map[string]*AppchainInfo),
		appchainID: appchainID,
	}
}

func (c *DirectChecker) BasicCheck(ibtp *pb.IBTP) (bool, error) {
	if err := ibtp.CheckServiceID(); err != nil {
		return false, err
	}

	bxhID0, chainID0, _ := ibtp.ParseFrom()
	bxhID1, chainID1, _ := ibtp.ParseTo()
	if bxhID0 != "" || bxhID1 != "" {
		return false, fmt.Errorf("invalid IBTP ID %s: bxh ID should be empty", ibtp.ID())
	}

	if chainID0 == chainID1 {
		return false, fmt.Errorf("invalid IBTP ID %s: the same from and to appchain", ibtp.ID())
	}

	if ibtp.Type == pb.IBTP_INTERCHAIN && c.appchainID == chainID1 {
		return true, nil
	}

	if (ibtp.Type == pb.IBTP_RECEIPT_SUCCESS || ibtp.Type == pb.IBTP_RECEIPT_FAILURE || ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK_END) && c.appchainID == chainID0 {
		return false, nil
	}

	if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK && c.appchainID == chainID0 {
		return false, nil
	}

	if ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK && c.appchainID == chainID1 {
		return true, nil
	}

	return false, fmt.Errorf("invalid IBTP ID %s with type %v", ibtp.ID(), ibtp.Type)
}

func (c *DirectChecker) CheckProof(ibtp *pb.IBTP) error {
	var chainID string

	_, chainID0, _, _ := pb.ParseFullServiceID(ibtp.From)
	_, chainID1, _, _ := pb.ParseFullServiceID(ibtp.To)

	if c.appchainID == chainID0 {
		chainID = chainID1
	} else {
		chainID = chainID0
	}

	appchainInfo, ok := c.chainInfoM[chainID]
	if !ok {
		broker, trustRoot, ruleAddr, err := c.client.GetAppchainInfo(chainID)
		if err != nil {
			return err
		}
		appchainInfo = &AppchainInfo{
			broker:    broker,
			trustRoot: trustRoot,
			ruleAddr:  strings.ToLower(ruleAddr),
		}
		c.chainInfoM[chainID] = appchainInfo
	}

	// todo: need validate direct mode
	ok, _, err := c.ve.Validate(appchainInfo.ruleAddr, chainID, ibtp.Proof, ibtp.Payload, string(appchainInfo.trustRoot))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("validate ibtp %s failed", ibtp.ID())
	}
	return nil
}

type CodeLedger struct {
	client       agency.Client
	validateCode map[string][]byte
}

func (l *CodeLedger) GetCode(address *types.Address) []byte {
	code, _ := l.validateCode[address.String()]
	return code
}

func (l *CodeLedger) SetCode(address *types.Address, code []byte) error {
	l.validateCode[address.String()] = code
	return nil
}
