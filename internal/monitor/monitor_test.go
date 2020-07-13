package monitor

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto/mock_txcrypto"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	fid  = "0x298935a2a08c9c7707dca2b4b95c6c8205ff1329"
	tid  = "mychannel-Transfer-001"
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to   = "0x4d936bf4990d9a28fa61510384400c1c301b2582"
	hash = "0x4d936bf4990d9a28fa61510384400c1c301b2582"
	name = "Alice"
)

func TestHandleIBTP(t *testing.T) {
	// set up new monitor
	mockClient, mnt := prepare(t)

	h := types.Hash{}
	h.SetBytes([]byte(hash))
	//receipt := &pb.Receipt{
	//	Version: []byte("0.4.1"),
	//	TxHash:  h,
	//	Ret:     nil,
	//	Status:  0,
	//	Events:  nil,
	//}
	queryIbtp, err := createIBTP(2, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	require.Nil(t, err)

	ibtpCh := make(chan *pb.IBTP, 2)

	mockClient.EXPECT().CommitCallback(gomock.Any()).Return(nil).AnyTimes()
	mockClient.EXPECT().GetOutMessage(gomock.Any(), gomock.Any()).Return(queryIbtp, nil)
	mockClient.EXPECT().Start().Return(nil).AnyTimes()
	mockClient.EXPECT().Stop().Return(nil).AnyTimes()
	mockClient.EXPECT().GetIBTP().Return(ibtpCh).AnyTimes()

	//start appchain monitor
	require.Nil(t, mnt.Start())

	// handle correct sequence number ibtp
	ibtp, err := createIBTP(1, pb.IBTP_INTERCHAIN, "set", name, "")
	require.Nil(t, err)
	ibtpCh <- ibtp

	time.Sleep(500 * time.Millisecond)
	meta := mnt.QueryLatestMeta()
	require.Equal(t, uint64(1), meta[to])

	recv, err := mnt.QueryIBTP(queryIbtp.ID())
	require.Nil(t, err)
	require.Equal(t, queryIbtp, recv)

	require.Nil(t, mnt.Stop())
}

func prepare(t *testing.T) (*mock_client.MockClient, *AppchainMonitor) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockClient := mock_client.NewMockClient(mockCtl)
	mockCryptor := mock_txcrypto.NewMockCryptor(mockCtl)

	mockClient.EXPECT().GetOutMeta().Return(make(map[string]uint64), nil).AnyTimes()

	mnt, err := New(mockClient, mockCryptor)
	require.Nil(t, err)
	return mockClient, mnt
}

func createIBTP(idx uint64, typ pb.IBTP_Type, funct string, args string, callback string) (*pb.IBTP, error) {
	ct := pb.Content{
		SrcContractId: fid,
		DstContractId: tid,
		Func:          funct,
		Args:          [][]byte{[]byte(args)},
		Callback:      callback,
	}
	c, err := ct.Marshal()
	if err != nil {
		return nil, err
	}
	pd := pb.Payload{
		Encrypted: false,
		Content:   c,
	}
	b, err := pd.Marshal()
	if err != nil {
		return nil, err
	}

	return &pb.IBTP{
		From:      from,
		To:        to,
		Index:     idx,
		Type:      typ,
		Timestamp: time.Now().UnixNano(),
		Payload:   b,
		Version:   "0.4.1",
	}, nil
}
