package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	from = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

func TestAppchain(t *testing.T) {
	ag, mockClient := prepare(t)

	// set up return receipt
	chainInfo := &rpcx.Appchain{
		ID:            from,
		Name:          "fabric",
		Validators:    "fabric",
		ConsensusType: 0,
		Status:        0,
		ChainType:     "fabric",
	}
	b, err := json.Marshal(chainInfo)
	require.Nil(t, err)

	receipt := &pb.Receipt{
		Ret:    b,
		Status: 0,
	}
	mockClient.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any()).Return(receipt, nil)
	chain, err := ag.Appchain()
	require.Nil(t, err)
	require.Equal(t, chainInfo, chain)
}

func TestSyncBlock(t *testing.T) {
	ag, mockClient := prepare(t)

	hash := types.String2Hash(from)
	header := &pb.BlockHeader{
		Timestamp: time.Now().UnixNano(),
	}
	wrapper := &pb.InterchainTxWrapper{
		TransactionHashes: []types.Hash{hash, hash},
	}

	txWrappers := make([]*pb.InterchainTxWrapper, 0)
	txWrappers = append(txWrappers, wrapper)
	wrappers := &pb.InterchainTxWrappers{
		InterchainTxWrappers: txWrappers,
	}
	subHeaderCh := make(chan interface{}, 1)
	syncHeaderCh := make(chan *pb.BlockHeader, 1)
	subWrapperCh := make(chan interface{}, 1)
	syncWrapperCh := make(chan *pb.InterchainTxWrappers, 1)
	getHeaderCh := make(chan *pb.BlockHeader, 1)
	getWrapperCh := make(chan *pb.InterchainTxWrappers, 1)

	subHeaderCh <- header
	subWrapperCh <- wrapper

	mockClient.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_BLOCK_HEADER, gomock.Any()).Return(subHeaderCh, nil).AnyTimes()
	mockClient.EXPECT().Subscribe(gomock.Any(), pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER, gomock.Any()).Return(subWrapperCh, nil).AnyTimes()
	mockClient.EXPECT().GetInterchainTxWrappers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockClient.EXPECT().GetBlockHeader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	require.Nil(t, ag.SyncBlockHeader(context.Background(), syncHeaderCh))
	require.Nil(t, ag.SyncInterchainTxWrappers(context.Background(), syncWrapperCh))

	require.Equal(t, header, <-syncHeaderCh)
	require.Equal(t, wrappers, <-syncWrapperCh)

	getWrapperCh <- wrappers
	getHeaderCh <- header

	require.Nil(t, ag.GetBlockHeader(context.Background(), 1, 2, getHeaderCh))
	require.Nil(t, ag.GetInterchainTxWrappers(context.Background(), 1, 2, getWrapperCh))

	require.Equal(t, header, <-getHeaderCh)
	require.Equal(t, wrappers, <-getWrapperCh)
}

func TestSendTransaction(t *testing.T) {
	ag, mockClient := prepare(t)

	b := types.Address{}
	b.SetBytes([]byte(from))
	tx := &pb.Transaction{
		From: b,
	}
	r := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: 0,
	}

	mockClient.EXPECT().SendTransactionWithReceipt(gomock.Any()).Return(r, nil)
	receipt, err := ag.SendTransaction(tx)
	require.Nil(t, err)
	require.Equal(t, r, receipt)
}

func TestSendIBTP(t *testing.T) {
	ag, mockClient := prepare(t)

	r := &pb.Receipt{
		Ret:    []byte("this is a test"),
		Status: 0,
	}
	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()

	receipt, err := ag.SendIBTP(&pb.IBTP{})
	require.Nil(t, err)
	require.Equal(t, r, receipt)
}

func TestGetIBTPByID(t *testing.T) {
	ag, mockClient := prepare(t)

	r := &pb.Receipt{
		Ret:    []byte(from),
		Status: 0,
	}
	origin := &pb.IBTP{
		From:      from,
		Index:     1,
		Timestamp: time.Now().UnixNano(),
	}
	ib, err := origin.Marshal()
	require.Nil(t, err)

	tmpIP := &pb.InvokePayload{
		Method: "set",
		Args:   []*pb.Arg{{Value: ib}},
	}
	pd, err := tmpIP.Marshal()
	require.Nil(t, err)

	data := &pb.TransactionData{
		Payload: pd,
	}

	tx := &pb.Transaction{
		Data: data,
	}
	mockClient.EXPECT().InvokeContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(r, nil).AnyTimes()
	mockClient.EXPECT().GetTransaction(gomock.Any()).Return(&pb.GetTransactionResponse{Tx: tx}, nil)

	ibtp, err := ag.GetIBTPByID(from)
	require.Nil(t, err)
	require.Equal(t, origin, ibtp)
}

func prepare(t *testing.T) (*BxhAgent, *mock_client.MockClient) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	mockClient := mock_client.NewMockClient(mockCtl)

	addr := types.Address{}
	addr.SetBytes([]byte(from))
	bitxhub := repo.Relay{
		Addr: "localhost:60011",
		Validators: []string{
			"0x000f1a7a08ccc48e5d30f80850cf1cf283aa3abd",
			"0xe93b92f1da08f925bdee44e91e7768380ae83307",
			"0xb18c8575e3284e79b92100025a31378feb8100d6",
			"0x856E2B9A5FA82FD1B031D1FF6863864DBAC7995D",
		},
	}

	ag, err := New(mockClient, addr, bitxhub)
	require.Nil(t, err)
	return ag, mockClient
}
