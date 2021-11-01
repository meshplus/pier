package appchain_adapter

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/meshplus/pier/internal/adapt"

	"github.com/meshplus/pier/internal/repo"

	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/checker/mock_checker"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/txcrypto/mock_txcrypto"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
)

const (
	appchain  = "appchain"
	appchain1 = "appchain1"
	appchain2 = "appchain2"
	bxhID     = "1356"
	bxhID1    = "1357"
)

func TestAppchainAdapter_GetAppchainID(t *testing.T) {
	appchainAdapter, _, _, _ := mockAppchainAdapter(t)

	appchainID := appchainAdapter.ID()
	require.Equal(t, appchain, appchainID)
}

func TestAppchainAdapter_GetServiceIDList(t *testing.T) {
	appchainAdapter, client, _, _ := mockAppchainAdapter(t)

	client.EXPECT().GetServices().Return(nil, fmt.Errorf("not found")).MaxTimes(1)
	serviceList, err := appchainAdapter.GetServiceIDList()
	require.NotNil(t, err)
	require.Nil(t, serviceList)

	services := []string{"service0", "service1"}
	client.EXPECT().GetServices().Return(services, nil).MaxTimes(1)
	serviceList, err = appchainAdapter.GetServiceIDList()
	require.Nil(t, err)
	require.Equal(t, services, serviceList)
}

func TestAppchainAdapter_MonitorIBTP(t *testing.T) {
	appchainAdapter, _, _, _ := mockAppchainAdapter(t)

	ibtpC := appchainAdapter.MonitorIBTP()
	require.NotNil(t, ibtpC)
}

func TestAppchainAdapter_MonitorUpdatedMeta(t *testing.T) {

}

func TestAppchainAdapter_Name(t *testing.T) {
	appchainAdapter, _, _, _ := mockAppchainAdapter(t)

	name := appchainAdapter.Name()
	require.Equal(t, fmt.Sprintf("appchain:%s", appchain), name)
}

func TestAppchainAdapter_QueryIBTP(t *testing.T) {
	appchainAdapter, client, _, _ := mockAppchainAdapter(t)

	ibtp, err := appchainAdapter.QueryIBTP("", true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	from := fmt.Sprintf("1356:%s:service1", appchain1)
	to := fmt.Sprintf("1356:%s:service2", appchain2)
	servicePair := fmt.Sprintf("%s-%s", from, to)
	ibtpExp := &pb.IBTP{
		From:  from,
		To:    to,
		Index: 1,
	}
	client.EXPECT().GetOutMessage(servicePair, uint64(1)).Return(ibtpExp, nil).MaxTimes(1)
	client.EXPECT().GetReceiptMessage(servicePair, uint64(1)).Return(ibtpExp, nil).MaxTimes(1)

	id := fmt.Sprintf("%s-%s-%d", from, to, 1)
	ibtp, err = appchainAdapter.QueryIBTP(id, true)
	require.Nil(t, err)
	require.Equal(t, ibtpExp, ibtp)

	ibtp, err = appchainAdapter.QueryIBTP(id, false)
	require.Nil(t, err)
	require.Equal(t, ibtpExp, ibtp)

	client.EXPECT().GetOutMessage(servicePair, uint64(2)).Return(nil, errors.New("not found")).AnyTimes()
	client.EXPECT().GetReceiptMessage(servicePair, uint64(2)).Return(nil, errors.New("not found")).AnyTimes()
	id = fmt.Sprintf("%s-%s-%d", from, to, 2)
	ibtp, err = appchainAdapter.QueryIBTP(id, true)
	require.NotNil(t, err)
	require.Nil(t, ibtp)

	ibtp, err = appchainAdapter.QueryIBTP(id, false)
	require.NotNil(t, err)
	require.Nil(t, ibtp)
}

func TestAppchainAdapter_QueryInterchain(t *testing.T) {
	appchainAdapter, client, _, _ := mockAppchainAdapter(t)
	srcService0 := fmt.Sprintf("%s:%s:service0", bxhID, appchain)
	srcService1 := fmt.Sprintf("%s:%s:service1", bxhID, appchain)
	dstService0 := fmt.Sprintf("%s:%s:service0", bxhID, appchain1)
	dstService1 := fmt.Sprintf("%s:%s:service1", bxhID, appchain1)
	src0dst0 := fmt.Sprintf("%s-%s", srcService0, dstService0)
	src0dst1 := fmt.Sprintf("%s-%s", srcService0, dstService1)
	src1dst0 := fmt.Sprintf("%s-%s", srcService1, dstService0)
	src1dst1 := fmt.Sprintf("%s-%s", srcService1, dstService1)

	outMata := map[string]uint64{
		src0dst0: 1,
		src0dst1: 2,
		src1dst0: 3,
		src1dst1: 4,
	}

	callbackMeta := map[string]uint64{
		src0dst1: 2,
		src1dst0: 2,
		src1dst1: 4,
	}

	inMeta := map[string]uint64{
		fmt.Sprintf("%s-%s", dstService0, srcService0): 1,
		fmt.Sprintf("%s-%s", dstService1, srcService1): 2,
	}

	interchainExp := &pb.Interchain{
		ID:                      srcService0,
		InterchainCounter:       map[string]uint64{dstService0: 1, dstService1: 2},
		ReceiptCounter:          map[string]uint64{dstService1: 2},
		SourceInterchainCounter: map[string]uint64{dstService0: 1},
		SourceReceiptCounter:    map[string]uint64{dstService0: 1},
	}

	client.EXPECT().GetOutMeta().Return(nil, errors.New("")).MaxTimes(1)
	interchain, err := appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetOutMeta().Return(map[string]uint64{"": 1}, nil).MaxTimes(3)
	client.EXPECT().GetCallbackMeta().Return(nil, errors.New("")).MaxTimes(1)
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetCallbackMeta().Return(map[string]uint64{"": 1}, nil).MaxTimes(3)
	client.EXPECT().GetInMeta().Return(nil, errors.New("")).MaxTimes(1)
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetInMeta().Return(map[string]uint64{"": 1}, nil).MaxTimes(3)
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetOutMeta().Return(outMata, nil).AnyTimes()
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetCallbackMeta().Return(callbackMeta, nil).AnyTimes()
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.NotNil(t, err)
	require.Nil(t, interchain)

	client.EXPECT().GetInMeta().Return(inMeta, nil).AnyTimes()
	interchain, err = appchainAdapter.QueryInterchain(srcService0)
	require.Nil(t, err)
	require.Equal(t, interchainExp, interchain)
}

func TestAppchainAdapter_SendIBTP(t *testing.T) {
	appchainAdapter, client, cryptor, checker := mockAppchainAdapter(t)

	checker.EXPECT().BasicCheck(gomock.Any()).DoAndReturn(func(ibtp *pb.IBTP) (bool, error) {
		if ibtp.Index == 0 {
			return false, errors.New("")
		}
		return ibtp.Category() == pb.IBTP_REQUEST, nil
	}).AnyTimes()

	contentDecFail, payloadDecFail := genContentPayload(t, "decFail", true, false)
	contentEnc, payloadEnc := genContentPayload(t, "set", true, false)
	_, payload := genContentPayload(t, "set", false, false)
	_, payloadBad := genContentPayload(t, "set", false, true)
	_, payloadResultBad := genResultPayload(t, []byte{1, 2, 3}, true)
	_, payloadResult := genResultPayload(t, []byte{1, 2, 3}, false)
	cryptor.EXPECT().Decrypt(contentDecFail, gomock.Any()).Return(nil, errors.New("")).AnyTimes()
	cryptor.EXPECT().Decrypt(contentEnc, gomock.Any()).Return(contentEnc, nil).AnyTimes()

	ibtp := &pb.IBTP{
		From:    fmt.Sprintf("%s:%s:service0", bxhID, appchain1),
		To:      fmt.Sprintf("%s:%s:service1", bxhID, appchain),
		Index:   0,
		Payload: []byte{1},
	}
	client.EXPECT().SubmitIBTP(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(from string, index uint64, serviceID string, ibtpType pb.IBTP_Type, content *pb.Content, proof *pb.BxhProof, isEncrypted bool) (*pb.SubmitIBTPResponse, error) {
			if index == 3 {
				return nil, errors.New("")
			} else if index == 2 {
				return &pb.SubmitIBTPResponse{Status: false}, nil
			}
			return &pb.SubmitIBTPResponse{Status: true}, nil
		}).AnyTimes()
	client.EXPECT().SubmitReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(to string, index uint64, serviceID string, ibtpType pb.IBTP_Type, result *pb.Result, proof *pb.BxhProof) (*pb.SubmitIBTPResponse, error) {
			if index == 3 {
				return nil, errors.New("")
			} else if index == 2 {
				return &pb.SubmitIBTPResponse{Status: false}, nil
			}
			return &pb.SubmitIBTPResponse{Status: true}, nil
		}).AnyTimes()

	checker.EXPECT().CheckProof(gomock.Any()).DoAndReturn(func(ibtp *pb.IBTP) error {
		if ibtp.Proof == nil {
			return errors.New("")
		}
		return nil
	}).AnyTimes()

	err := appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Index = 1
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Payload = payloadDecFail
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Type = pb.IBTP_RECEIPT_SUCCESS
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Type = pb.IBTP_INTERCHAIN
	ibtp.Payload = payloadEnc
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Payload = payload
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Proof = []byte{1}
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	bxhProof := &pb.BxhProof{}
	proof, err := bxhProof.Marshal()
	require.Nil(t, err)
	ibtp.Proof = proof
	ibtp.Payload = payloadBad
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Payload = payload
	ibtp.Index = 3
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Index = 2
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Index = 1
	err = appchainAdapter.SendIBTP(ibtp)
	require.Nil(t, err)

	ibtp.Type = pb.IBTP_RECEIPT_SUCCESS
	ibtp.Payload = payloadResultBad
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Payload = payloadResult
	ibtp.Index = 3
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Index = 2
	err = appchainAdapter.SendIBTP(ibtp)
	require.NotNil(t, err)

	ibtp.Index = 1
	err = appchainAdapter.SendIBTP(ibtp)
	require.Nil(t, err)
}

func TestAppchainAdapter_SendUpdatedMeta(t *testing.T) {

}

func TestAppchainAdapter_Start(t *testing.T) {
	appchainAdapter, client, cryptor, _ := mockAppchainAdapter(t)
	c0 := client.EXPECT().Start().Return(errors.New("start failed")).MaxTimes(1)
	client.EXPECT().Start().Return(nil).AnyTimes().After(c0)

	ibtpC := make(chan *pb.IBTP, 1024)
	c1 := client.EXPECT().GetIBTPCh().Return(nil).MaxTimes(1)
	client.EXPECT().GetIBTPCh().Return(ibtpC).AnyTimes().After(c1)

	contentEncFail, payloadEncFail := genContentPayload(t, "decFail", true, false)
	contentEnc, payloadEnc := genContentPayload(t, "set", true, false)
	_, payload := genContentPayload(t, "set", false, false)
	cryptor.EXPECT().Encrypt(contentEncFail, gomock.Any()).Return(nil, errors.New("")).AnyTimes()
	cryptor.EXPECT().Encrypt(contentEnc, gomock.Any()).Return(contentEnc, nil).AnyTimes()

	err := appchainAdapter.Start()
	require.NotNil(t, err)

	err = appchainAdapter.Start()
	require.Nil(t, err)
	monitorC := appchainAdapter.MonitorIBTP()
	require.Equal(t, 0, len(monitorC))
	_, ok := <-monitorC
	require.False(t, ok)

	appchainAdapter.(*AppchainAdapter).ibtpC = make(chan *pb.IBTP, 1024)
	err = appchainAdapter.Start()
	require.Nil(t, err)
	ibtp := &pb.IBTP{
		From:    fmt.Sprintf("%s:%s:service0", bxhID, appchain1),
		To:      fmt.Sprintf("%s:%s:service1", bxhID, appchain),
		Index:   1,
		Payload: []byte{1},
	}
	ibtpC <- ibtp
	time.Sleep(time.Millisecond * 100)
	monitorC = appchainAdapter.MonitorIBTP()
	require.Equal(t, 0, len(monitorC))

	ibtp.Payload = payloadEncFail
	ibtpC <- ibtp
	time.Sleep(time.Millisecond * 100)
	require.Equal(t, 0, len(monitorC))

	ibtp.Type = pb.IBTP_RECEIPT_SUCCESS
	ibtpC <- ibtp
	time.Sleep(time.Millisecond * 100)
	require.Equal(t, 0, len(monitorC))

	ibtp.Type = pb.IBTP_INTERCHAIN
	ibtp.Payload = payloadEnc
	ibtpC <- ibtp
	time.Sleep(time.Millisecond * 100)
	require.Equal(t, 1, len(monitorC))
	<-monitorC

	ibtp.Payload = payload
	ibtpC <- ibtp
	time.Sleep(time.Millisecond * 100)
	require.Equal(t, 1, len(monitorC))
	<-monitorC

	close(ibtpC)
	time.Sleep(time.Millisecond * 100)
	require.Equal(t, 0, len(monitorC))
	_, ok = <-monitorC
	require.False(t, ok)
}

func TestAppchainAdapter_Stop(t *testing.T) {
	appchainAdapter, client, _, _ := mockAppchainAdapter(t)
	c0 := client.EXPECT().Stop().Return(errors.New("stop failed")).MaxTimes(1)
	client.EXPECT().Stop().Return(nil).AnyTimes().After(c0)

	err := appchainAdapter.Stop()
	require.NotNil(t, err)

	err = appchainAdapter.Stop()
	require.Nil(t, err)
	require.Nil(t, appchainAdapter.(*AppchainAdapter).client)
	require.Nil(t, appchainAdapter.(*AppchainAdapter).pluginClient)
}

func mockAppchainAdapter(t *testing.T) (adapt.Adapt, *mock_client.MockClient, *mock_txcrypto.MockCryptor, *mock_checker.MockChecker) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)
	mockCryptor := mock_txcrypto.NewMockCryptor(c)
	mockChecker := mock_checker.NewMockChecker(c)

	pluginClient := &plugin.Client{}

	return &AppchainAdapter{
		config:       repo.DefaultConfig(),
		client:       mockClient,
		pluginClient: pluginClient,
		checker:      mockChecker,
		cryptor:      mockCryptor,
		logger:       log.NewWithModule(loggers.App),
		ibtpC:        make(chan *pb.IBTP, 1024),
		appchainID:   appchain,
		bitxhubID:    bxhID,
	}, mockClient, mockCryptor, mockChecker
}

func genContentPayload(t *testing.T, function string, isEncrypt bool, badContent bool) ([]byte, []byte) {
	content := pb.Content{
		Func: function,
		Args: [][]byte{[]byte("Alice")},
	}

	ct, err := content.Marshal()
	require.Nil(t, err)

	if badContent {
		ct = []byte{1}
	}
	hash := sha256.Sum256(ct)

	payload := pb.Payload{
		Encrypted: isEncrypt,
		Content:   ct,
		Hash:      hash[:],
	}

	pd, err := payload.Marshal()
	require.Nil(t, err)

	return ct, pd
}

func genResultPayload(t *testing.T, data []byte, badResult bool) ([]byte, []byte) {
	result := pb.Result{
		Data: [][]byte{data},
	}

	ct, err := result.Marshal()
	require.Nil(t, err)

	if badResult {
		ct = []byte{1}
	}
	hash := sha256.Sum256(ct)

	payload := pb.Payload{
		Encrypted: false,
		Content:   ct,
		Hash:      hash[:],
	}

	pd, err := payload.Marshal()
	require.Nil(t, err)

	return ct, pd
}
