package checker

import (
	"fmt"
	"math"
	"testing"

	"github.com/meshplus/bitxhub-core/validator"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/internal/loggers"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	appchain0 = "appchain0"
	appchain1 = "appchain1"
	appchain2 = "appchain2"
	bxhID     = "1356"
	bxhID1    = "1357"
)

func TestRelayChecker_BasicCheck(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)

	checker := NewRelayChecker(mockClient, appchain1, bxhID, log.NewWithModule(loggers.App))

	ibtp := &pb.IBTP{
		From:  fmt.Sprintf("%s:%s:service0", bxhID, appchain0),
		To:    fmt.Sprintf("%s:%s:service1", bxhID, appchain1),
		Index: 1,
		Type:  pb.IBTP_INTERCHAIN,
	}
	isReq, err := checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.True(t, isReq)

	ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
	isReq, err = checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.True(t, isReq)

	ibtp.From = fmt.Sprintf("%s:%s:service0", bxhID, appchain1)
	ibtp.To = fmt.Sprintf("%s:%s:service1", bxhID, appchain0)
	ibtp.Type = pb.IBTP_INTERCHAIN
	isReq, err = checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.False(t, isReq)

	ibtp.To = fmt.Sprintf("%s:%s:service1", bxhID, appchain1)
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf("%s:%s:service0", bxhID, appchain0)
	ibtp.To = fmt.Sprintf("%s:%s:service1", bxhID1, appchain1)
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)
}

func TestRelayChecker_CheckProof(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)

	checker := NewRelayChecker(mockClient, appchain1, bxhID, log.NewWithModule(loggers.App))
	err := checker.CheckProof(nil)
	require.Nil(t, err)
}

func TestDirectChecker_BasicCheck(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)

	checker := NewDirectChecker(mockClient, appchain1, log.NewWithModule(loggers.App), math.MaxUint64)

	ibtp := &pb.IBTP{
		From:  fmt.Sprintf(":%s:service0", appchain0),
		To:    fmt.Sprintf(":%s:service1", appchain1),
		Index: 1,
		Type:  pb.IBTP_INTERCHAIN,
	}
	isReq, err := checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.True(t, isReq)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain1)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain0)
	ibtp.Type = pb.IBTP_RECEIPT_FAILURE
	isReq, err = checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.False(t, isReq)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain1)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain0)
	ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
	isReq, err = checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.False(t, isReq)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain0)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain1)
	ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK
	isReq, err = checker.BasicCheck(ibtp)
	require.Nil(t, err)
	require.True(t, isReq)

	ibtp.From = fmt.Sprintf("%s:%s:service0", bxhID, appchain1)
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain1)
	ibtp.To = fmt.Sprintf("%s:%s:service0", bxhID, appchain0)
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain1)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain1)
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain1)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain0)
	ibtp.Type = pb.IBTP_INTERCHAIN
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain0)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain1)
	ibtp.Type = pb.IBTP_RECEIPT_SUCCESS
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain0)
	ibtp.To = fmt.Sprintf(":%s:service1", appchain1)
	ibtp.Type = pb.IBTP_RECEIPT_ROLLBACK_END
	isReq, err = checker.BasicCheck(ibtp)
	require.NotNil(t, err)
}

func TestDirectChecker_CheckProof(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)

	appchainInfo := &AppchainInfo{
		broker:    "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997",
		trustRoot: []byte{1},
		ruleAddr:  validator.HappyRuleAddr,
	}
	mockClient.EXPECT().GetAppchainInfo(appchain0).Return(appchainInfo.broker, appchainInfo.trustRoot, appchainInfo.ruleAddr, nil).AnyTimes()

	checker := NewDirectChecker(mockClient, appchain1, log.NewWithModule(loggers.App), math.MaxUint64)

	ibtp := &pb.IBTP{
		From:  fmt.Sprintf(":%s:service0", appchain2),
		To:    fmt.Sprintf(":%s:service1", appchain1),
		Index: 1,
		Type:  pb.IBTP_INTERCHAIN,
	}
	mockClient.EXPECT().GetAppchainInfo(appchain2).Return("", nil, "", fmt.Errorf("no appchain found")).MaxTimes(1)
	err := checker.CheckProof(ibtp)
	require.Nil(t, err)

	mockClient.EXPECT().GetAppchainInfo(appchain2).Return("", nil, validator.FabricRuleAddr, nil).MaxTimes(1)
	err = checker.CheckProof(ibtp)
	require.NotNil(t, err)

	ibtp.From = fmt.Sprintf(":%s:service0", appchain0)
	err = checker.CheckProof(ibtp)
	require.Nil(t, err)

	err = checker.CheckProof(ibtp)
	require.Nil(t, err)
}
