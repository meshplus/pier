package monitor

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent/mock_agent"
	"github.com/meshplus/pier/pkg/plugins/client/mock_client"
	"github.com/stretchr/testify/require"
)

const (
	fid        = "0x298935a2a08c9c7707dca2b4b95c6c8205ff1329"
	tid        = "mychannel-Transfer-001"
	from       = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to         = "0x4d936bf4990d9a28fa61510384400c1c301b2582"
	hash       = "0x4d936bf4990d9a28fa61510384400c1c301b2582"
	validators = "0x000f1a7a08ccc48e5d30f80850cf1cf283aa3abd," +
		"0xe93b92f1da08f925bdee44e91e7768380ae83307"
	name = "Alice"
)

func TestHandleIBTP(t *testing.T) {
	// set up new monitor
	mockAgent, mockClient, mnt := prepare(t)

	h := types.Hash{}
	h.SetBytes([]byte(hash))
	receipt := &pb.Receipt{
		Version: []byte("0.4.1"),
		TxHash:  h,
		Ret:     nil,
		Status:  0,
		Events:  nil,
	}
	missingIbtp, err := createIBTP(2, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	require.Nil(t, err)

	ibtpCh := make(chan *pb.IBTP, 2)

	mockAgent.EXPECT().SendIBTP(gomock.Any()).Return(receipt, nil).AnyTimes()
	mockClient.EXPECT().CommitCallback(gomock.Any()).Return(nil).AnyTimes()
	mockClient.EXPECT().GetOutMessage(gomock.Any(), gomock.Any()).Return(missingIbtp, nil)
	mockClient.EXPECT().GetOutMeta().Return(make(map[string]uint64), nil).AnyTimes()
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
	require.Equal(t, uint64(1), mnt.meta.InterchainCounter[to])

	// handle missing ibtp index
	ibtp3, err := createIBTP(3, pb.IBTP_INTERCHAIN, "set", name, "")
	require.Nil(t, err)
	ibtpCh <- ibtp3

	time.Sleep(500 * time.Millisecond)
	require.Equal(t, uint64(3), mnt.meta.InterchainCounter[to])
	require.Nil(t, mnt.Stop())
}

func TestRecovery(t *testing.T) {
	// set up new monitor
	mockAgent, mockClient, mnt := prepare(t)

	h := types.Hash{}
	h.SetBytes([]byte(hash))
	successReceipt := &pb.Receipt{
		TxHash: h,
		Status: 0,
	}
	failedReceipt := &pb.Receipt{
		TxHash: h,
		Status: 1,
	}
	missingIbtp1, err := createIBTP(1, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	require.Nil(t, err)
	missingIbtp2, err := createIBTP(2, pb.IBTP_INTERCHAIN, "get", name, "setCallback")
	require.Nil(t, err)
	outMeta := make(map[string]uint64)
	outMeta[to] = 2

	mockAgent.EXPECT().SendIBTP(missingIbtp1).Return(successReceipt, nil).AnyTimes()
	mockAgent.EXPECT().SendIBTP(missingIbtp2).Return(failedReceipt, nil).AnyTimes()
	mockClient.EXPECT().CommitCallback(gomock.Any()).Return(nil).AnyTimes()
	mockClient.EXPECT().GetOutMeta().Return(outMeta, nil).AnyTimes()
	mockClient.EXPECT().GetOutMessage(to, uint64(1)).Return(missingIbtp1, nil).AnyTimes()
	mockClient.EXPECT().GetOutMessage(to, uint64(2)).Return(missingIbtp2, nil).AnyTimes()
	mockClient.EXPECT().Stop().Return(nil).AnyTimes()

	// invoke recovery
	err = mnt.recovery()
	require.Nil(t, err)
	require.Equal(t, uint64(1), mnt.meta.InterchainCounter[to])
	require.Nil(t, mnt.Stop())
}

func prepare(t *testing.T) (*mock_agent.MockAgent, *mock_client.MockClient, *AppchainMonitor) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockAgent := mock_agent.NewMockAgent(mockCtl)
	mockClient := mock_client.NewMockClient(mockCtl)
	meta := &rpcx.Appchain{
		ID:            from,
		Name:          "hyperchain",
		Validators:    validators,
		ConsensusType: 1,
		Status:        0,
		ChainType:     "hyperchain",
	}
	mnt, err := New(mockAgent, mockClient, meta)
	require.Nil(t, err)
	return mockAgent, mockClient, mnt
}

func createIBTP(idx uint64, typ pb.IBTP_Type, funct string, args string, callback string) (*pb.IBTP, error) {
	pd := pb.Payload{
		SrcContractId: fid,
		DstContractId: tid,
		Func:          funct,
		Args:          [][]byte{[]byte(args)},
		Callback:      callback,
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
