package checker

import (
	"fmt"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ Checker = (*RelayChecker)(nil)

type RelayChecker struct {
	client     plugins.Client
	chainInfoM map[string]*AppchainInfo
	bxhID      string
	appchainID string
}

func NewRelayChecker(client plugins.Client, appchainID, bxhID string, logger logrus.FieldLogger) Checker {
	return &RelayChecker{
		client:     client,
		bxhID:      bxhID,
		appchainID: appchainID,
	}
}

func (c *RelayChecker) BasicCheck(ibtp *pb.IBTP) (bool, error) {
	if err := ibtp.CheckServiceID(); err != nil {
		return false, err
	}

	bxhID0, chainID0, _ := ibtp.ParseFrom()
	bxhID1, chainID1, _ := ibtp.ParseTo()

	if bxhID0 == bxhID1 && chainID0 == chainID1 {
		return false, fmt.Errorf("invalid IBTP ID %s", ibtp.ID())
	}

	if (bxhID0 == c.bxhID || bxhID0 == "did:bitxhub") && chainID0 == c.appchainID {
		return false, nil
	}

	if (bxhID1 == c.bxhID || bxhID1 == "did:bitxhub") && chainID1 == c.appchainID {
		return true, nil
	}

	return false, fmt.Errorf("invalid IBTP ID %s with type %v", ibtp.ID(), ibtp.Type)
}

func (c *RelayChecker) CheckProof(ibtp *pb.IBTP) error {
	return nil
}
