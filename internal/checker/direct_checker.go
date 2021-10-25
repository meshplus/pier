package checker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/meshplus/bitxhub-core/validator"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ Checker = (*DirectChecker)(nil)

type DirectChecker struct {
	ve         *validator.ValidationEngine
	client     plugins.Client
	chainInfoM map[string]*AppchainInfo
	appchainID string
}

type AppchainInfo struct {
	broker    string
	trustRoot []byte
	ruleAddr  string
}

func NewDirectChecker(client plugins.Client, appchainID string, logger logrus.FieldLogger, gasLimit uint64) Checker {
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

	if ibtp.Category() == pb.IBTP_REQUEST && c.appchainID != chainID1 ||
		ibtp.Category() == pb.IBTP_RESPONSE && c.appchainID != chainID0 {
		return false, fmt.Errorf("invalid IBTP ID %s with type %v", ibtp.ID(), ibtp.Type)
	}

	return ibtp.Category() == pb.IBTP_REQUEST, nil
}

func (c *DirectChecker) CheckProof(ibtp *pb.IBTP) error {
	var chainID string

	if ibtp.Category() == pb.IBTP_REQUEST {
		_, chainID, _, _ = pb.ParseFullServiceID(ibtp.From)
	} else {
		_, chainID, _, _ = pb.ParseFullServiceID(ibtp.To)
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
	client       plugins.Client
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
