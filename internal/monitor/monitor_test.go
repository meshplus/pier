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
	ignoreIbtp, err := createIBTP(2, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	require.Nil(t, err)
	missingIbtp, err := createIBTP(3, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	normalIbtp, err := createIBTP(4, pb.IBTP_INTERCHAIN, "get", name, "setCallback")

	ibtpCh := make(chan *pb.IBTP, 2)

	mockClient.EXPECT().CommitCallback(gomock.Any()).Return(nil).AnyTimes()
	mockClient.EXPECT().GetOutMessage(gomock.Any(), gomock.Any()).Return(missingIbtp, nil).AnyTimes()
	mockClient.EXPECT().Start().Return(nil).AnyTimes()
	mockClient.EXPECT().Stop().Return(nil).AnyTimes()
	mockClient.EXPECT().GetIBTP().Return(ibtpCh).AnyTimes()

	//start appchain monitor
	require.Nil(t, mnt.Start())

	// handle should ignored ibtp
	ibtpCh <- ignoreIbtp
	ibtpCh <- normalIbtp

	time.Sleep(500 * time.Millisecond)
	// check if latest out meta is 4
	meta := mnt.QueryLatestMeta()
	require.Equal(t, uint64(4), meta[to])

	recv := mnt.ListenOnIBTP()
	// check if missed and normal ibtp is handled
	require.Equal(t, 2, len(recv))
	close(mnt.recvCh)
	require.Equal(t, missingIbtp, <-recv)
	require.Equal(t, normalIbtp, <-recv)

	// check if missed ibtp is stored
	recvd, err := mnt.QueryIBTP(missingIbtp.ID())
	require.Nil(t, err)
	require.Equal(t, missingIbtp, recvd)

	require.Nil(t, mnt.Stop())
}

func prepare(t *testing.T) (*mock_client.MockClient, *AppchainMonitor) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockClient := mock_client.NewMockClient(mockCtl)
	mockCryptor := mock_txcrypto.NewMockCryptor(mockCtl)
	meta := map[string]uint64{to: 2}
	mockClient.EXPECT().GetOutMeta().Return(meta, nil).AnyTimes()

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
