package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/stretchr/testify/require"
)

func TestClient_SubmitIBTP(t *testing.T) {
	ag, mockClient := prepare(t)
	agClient := CreateClient(ag)

	r := &pb.Receipt{
		Ret:    []byte(nil),
		Status: 0,
	}
	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()

	ibtp := &pb.IBTP{
		From:      from,
		To:        to,
		Index:     1,
		Timestamp: time.Now().UnixNano(),
	}
	_, err := agClient.SubmitIBTP(ibtp)
	require.Nil(t, err)
}

func TestClient_GetInMeta(t *testing.T) {
	ag, mockClient := prepare(t)
	agClient := CreateClient(ag)
	mockInCounterMap := make(map[string]uint64)
	data, err := json.Marshal(mockInCounterMap)
	require.Nil(t, err)
	r := &pb.Receipt{
		Ret:    data,
		Status: 0,
	}
	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()
	_, err = agClient.GetInMeta()
	require.Nil(t, err)
}

func TestClient_Other(t *testing.T) {
	ag, _ := prepare(t)
	agClient := CreateClient(ag)

	_, err := agClient.GetInMessage("", uint64(1))
	require.Nil(t, err)

	_, err = agClient.GetCallbackMeta()
	require.Nil(t, err)

	err = agClient.Initialize("", "", []byte{0})
	require.Nil(t, err)

	err = agClient.Start()
	require.Nil(t, err)

	err = agClient.Stop()
	require.Nil(t, err)

	_ = agClient.GetIBTP()

	_, err = agClient.GetOutMeta()
	require.Nil(t, err)

	_, err = agClient.GetOutMessage(to, uint64(1))
	require.Nil(t, err)

	err = agClient.CommitCallback(&pb.IBTP{})
	require.Nil(t, err)

	_ = agClient.Name()
	_ = agClient.Type()

}
