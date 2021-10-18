package monitor

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto/mock_txcrypto"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	fid  = "0x298935A2a08C9C7707DcA2B4B95c6C8205ff1329"
	tid  = "mychannel-Transfer-001"
	from = "0x3f9d18f7C3a6E5E4C0B877FE3E688aB08840b997"
	to   = "0x4d936bf4990d9A28fa61510384400C1c301b2582"
	hash = "0x9f41dd84524bf8a42f8ab58ecfca6e1752d6fd93fe8dc00af4c71963c97db59f"
	name = "Alice"
)

//func TestHandleIBTP(t *testing.T) {
//	// set up new monitor
//	mockClient, mockCryptor, mnt := prepare(t)
//
//	h := types.Hash{}
//	h.SetBytes([]byte(hash))
//	normalIbtp, err := createIBTP(3, pb.IBTP_INTERCHAIN, "get", name, "setCallback", true)
//	//encryptedIbtp, err := createEncryptedIBTP(5, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
//	encryptedContent := []byte("encryptedContent")
//	ibtpCh := make(chan *pb.IBTP, 2)
//
//	originalMeta := map[string]uint64{to: 2}
//
//	mockClient.EXPECT().GetOutMessage(fmt.Sprintf("%s-%s", normalIbtp.From, normalIbtp.To), normalIbtp.Index).Return(normalIbtp, nil).MaxTimes(2)
//	mockClient.EXPECT().GetOutMeta().Return(originalMeta, nil)
//	//mockClient.EXPECT().GetOutMessage(to, uint64(6)).Return(nil, errWrongIBTP).AnyTimes()
//	mockClient.EXPECT().Start().Return(nil).AnyTimes()
//	mockClient.EXPECT().Stop().Return(nil).AnyTimes()
//	mockClient.EXPECT().GetIBTP().Return(ibtpCh).AnyTimes()
//	mockCryptor.EXPECT().Encrypt(gomock.Any(), gomock.Any()).Return(encryptedContent, nil).MaxTimes(3)
//	//start appchain monitor
//	require.Nil(t, mnt.Start())
//
//	ibtpCh <- normalIbtp
//	time.Sleep(500 * time.Millisecond)
//	// check if latest out meta is 4
//	meta := mnt.QueryOuterMeta()
//	require.Equal(t, uint64(2), meta[to])
//
//	recv := mnt.ListenIBTP()
//	// check if normal ibtp is handled
//	require.Equal(t, 1, len(recv))
//	require.Equal(t, normalIbtp, <-recv)
//	require.Equal(t, 0, len(recv))
//
//	// check if normal ibtp is stored
//	recvd, err := mnt.QueryIBTP(normalIbtp.ID())
//	require.Nil(t, err)
//	require.Equal(t, normalIbtp, recvd)
//
//	// test query ibtp with encryption
//	recvedEncrypIbtp, err := mnt.QueryIBTP(normalIbtp.ID())
//	require.Nil(t, err)
//	require.Equal(t, normalIbtp, recvedEncrypIbtp)
//
//	// test bad encrypted ibtp
//	mockCryptor.EXPECT().Encrypt(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("bad encrypted content"))
//	badEncryptedIbtp, err := createBadEncryptedIBTP(4, pb.IBTP_INTERCHAIN)
//	require.Nil(t, err)
//	ibtpCh <- badEncryptedIbtp
//	time.Sleep(500 * time.Millisecond)
//	close(mnt.recvCh)
//	// check if bad content ibtp is ignored
//	require.Equal(t, 0, len(recv))
//
//	// test query ibtp with error
//	mockCryptor.EXPECT().Encrypt(gomock.Any(), gomock.Any()).Return(encryptedContent, nil).AnyTimes()
//	badCall := mockClient.EXPECT().GetOutMessage(fmt.Sprintf("%s-%s", badEncryptedIbtp.From, badEncryptedIbtp.To), badEncryptedIbtp.Index).Return(nil, fmt.Errorf("worong call to get ibtp"))
//	badCall1 := mockClient.EXPECT().GetOutMessage(fmt.Sprintf("%s-%s", badEncryptedIbtp.From, badEncryptedIbtp.To), badEncryptedIbtp.Index).Return(badEncryptedIbtp, nil)
//	gomock.InOrder(badCall, badCall1)
//	recvd, err = mnt.QueryIBTP(badEncryptedIbtp.ID())
//	require.Nil(t, err)
//	require.Equal(t, badEncryptedIbtp, recvd)
//
//	require.Nil(t, mnt.Stop())
//}

func prepare(t *testing.T) (*mock_client.MockClient, *mock_txcrypto.MockCryptor, *AppchainMonitor) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockClient := mock_client.NewMockClient(mockCtl)
	mockCryptor := mock_txcrypto.NewMockCryptor(mockCtl)
	meta := map[string]uint64{to: 2}
	mockClient.EXPECT().GetOutMeta().Return(meta, nil).AnyTimes()

	mnt, err := New(mockClient, mockCryptor, log.NewWithModule("monitor"))
	require.Nil(t, err)
	return mockClient, mockCryptor, mnt
}

func createIBTP(idx uint64, typ pb.IBTP_Type, funct string, args string, callback string, encrypted bool) (*pb.IBTP, error) {
	ct := pb.Content{
		Func: funct,
		Args: [][]byte{[]byte(args)},
	}
	c, err := ct.Marshal()
	if err != nil {
		return nil, err
	}
	pd := pb.Payload{
		Encrypted: encrypted,
		Content:   c,
	}
	b, err := pd.Marshal()
	if err != nil {
		return nil, err
	}

	return &pb.IBTP{
		From:    from,
		To:      to,
		Index:   idx,
		Type:    typ,
		Payload: b,
		Version: "0.4.1",
	}, nil
}

func createBadEncryptedIBTP(idx uint64, typ pb.IBTP_Type) (*pb.IBTP, error) {
	pd := pb.Payload{
		Encrypted: true,
		Content:   []byte("bad encrypted content"),
	}
	b, err := pd.Marshal()
	if err != nil {
		return nil, err
	}

	return &pb.IBTP{
		From:    from,
		To:      to,
		Index:   idx,
		Type:    typ,
		Payload: b,
		Version: "0.4.1",
	}, nil
}
