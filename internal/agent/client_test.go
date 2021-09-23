package agent

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to   = "0x703b22368195d5063C5B5C26019301Cf2EbC83e2"
)

func TestClient_SubmitIBTP(t *testing.T) {
	mockClient := prepare(t)
	agClient := CreateClient(mockClient)

	r := &pb.Receipt{
		Ret:    []byte(nil),
		Status: 0,
	}
	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()

	ibtp := &pb.IBTP{
		From:  from,
		To:    to,
		Index: 1,
	}
	_, err := agClient.SubmitIBTP(ibtp)
	require.Nil(t, err)
}

func TestClient_SubmitIBTPWithCallback(t *testing.T) {
	mockClient := prepare(t)
	agClient := CreateClient(mockClient)

	r := &pb.Receipt{
		Ret:    []byte(nil),
		Status: 0,
	}

	sr := &pb.SignResponse{
		Sign: map[string][]byte{
			"0x123": []byte("0000000000000000000"),
		},
	}

	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()
	mockClient.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(sr, nil).AnyTimes()

	content := &pb.Content{
		Func: "get",
	}

	c, err := content.Marshal()
	require.Nil(t, err)

	pl := &pb.Payload{
		Encrypted: false,
		Content:   c,
	}

	p, err := pl.Marshal()
	require.Nil(t, err)

	ibtp := &pb.IBTP{
		From:    from,
		To:      to,
		Index:   1,
		Payload: p,
	}

	_, err = agClient.SubmitIBTP(ibtp)
	require.Nil(t, err)
}

func TestClient_GetInMeta(t *testing.T) {
	mockClient := prepare(t)
	agClient := CreateClient(mockClient)
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
	mockClient := prepare(t)
	agClient := CreateClient(mockClient)

	_, err := agClient.GetInMessage("", uint64(1))
	require.Nil(t, err)

	_, err = agClient.GetCallbackMeta()
	require.Nil(t, err)

	err = agClient.Initialize("", []byte{0})
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

func prepare(t *testing.T) *mock_client.MockClient {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	mockClient := mock_client.NewMockClient(mockCtl)
	addr := types.Address{}
	addr.SetBytes([]byte(from))
	mockClient.EXPECT().GetMultiSigns(gomock.Any(), gomock.Any()).Return(&pb.SignResponse{Sign: make(map[string][]byte)}, nil).AnyTimes()

	return mockClient
}
