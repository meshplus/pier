package plugins

import (
	"context"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGrpcServerAll(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	grpcServer := GRPCServer{cli}

	cli.EXPECT().GetOffChainData(gomock.Any()).Return(nil, nil).AnyTimes()
	_, err := grpcServer.GetOffChainData(ctx, nil)
	require.Nil(t, err)

	require.Panics(t, func() {
		grpcServer.GetOffChainDataReq(nil, nil)
	})
	require.Panics(t, func() {
		grpcServer.GetUpdateMeta(nil, nil)
	})

	cli.EXPECT().GetOffChainData(gomock.Any()).Return(nil, nil).AnyTimes()
	_, err = grpcServer.GetOffChainData(ctx, nil)
	require.Nil(t, err)

	cli.EXPECT().SubmitOffChainData(gomock.Any()).Return(nil).AnyTimes()
	_, err = grpcServer.SubmitOffChainData(nil, nil)
	require.Nil(t, err)

	cli.EXPECT().GetDstRollbackMeta().Return(nil, nil).AnyTimes()
	_, err = grpcServer.GetDstRollbackMeta(ctx, nil)
	require.Nil(t, err)

	cli.EXPECT().GetAppchainInfo(gomock.Any()).Return("", nil, "", nil).AnyTimes()
	req := &pb.ChainInfoRequest{
		ChainID: "fabric",
	}
	_, err = grpcServer.GetAppchainInfo(ctx, req)
	require.Nil(t, err)

	cli.EXPECT().GetChainID().Return("", "", nil).AnyTimes()
	_, err = grpcServer.GetChainID(ctx, nil)
	require.Nil(t, err)

	cli.EXPECT().GetDirectTransactionMeta(gomock.Any()).Return(uint64(0), uint64(0), uint64(0), nil).AnyTimes()
	direct := &pb.DirectTransactionMetaRequest{
		IBTPid: "1",
	}
	_, err = grpcServer.GetDirectTransactionMeta(ctx, direct)
	require.Nil(t, err)

	cli.EXPECT().SubmitIBTPBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	temp := &pb.SubmitIBTPRequestBatch{
		From:  []string{from},
		Index: []uint64{1, 2, 3},
	}
	_, err = grpcServer.SubmitIBTPBatch(ctx, temp)
	require.Nil(t, err)

	cli.EXPECT().SubmitReceiptBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	tt := &pb.SubmitReceiptRequestBatch{
		To:    []string{to},
		Index: []uint64{1, 2, 3},
	}
	_, err = grpcServer.SubmitReceiptBatch(ctx, tt)
	require.Nil(t, err)
}

func TestGrpcClientAll(t *testing.T) {
	ctx := context.Background()
	grpcClient, err := getGRPCClient(ctx, &mockAppchainPluginClient{})
	require.Nil(t, err)

	ctx1, _ := context.WithCancel(ctx)
	grpcClientError, err := getGRPCClient(ctx1, &mockAppchainPluginClient{})
	require.Nil(t, err)

	req := &pb.GetDataRequest{
		Index: uint64(1),
		From:  fmt.Sprintf("%s", "{#from}"),
		To:    fmt.Sprintf("%s", "{#to}"),
		Req:   nil,
	}
	_, err = grpcClient.GetOffChainData(req)
	require.Nil(t, err)

	res := &pb.GetDataResponse{
		Index: uint64(1),
		From:  fmt.Sprintf("%s", "{#from}"),
		To:    fmt.Sprintf("%s", "{#to}"),
		Data:  nil,
	}
	err = grpcClient.SubmitOffChainData(res)
	require.Nil(t, err)

	_, err = grpcClient.GetDstRollbackMeta()
	require.Nil(t, err)

	_, err = grpcClientError.GetDstRollbackMeta()
	require.NotNil(t, err)

	_, err = grpcClient.GetServices()
	require.Nil(t, err)

	_, err = grpcClientError.GetServices()
	require.NotNil(t, err)

	_, _, _, err = grpcClient.GetAppchainInfo("farbic")
	require.Nil(t, err)

	_, _, _, err = grpcClientError.GetAppchainInfo("farbic")
	require.NotNil(t, err)

	_, _, err = grpcClient.GetChainID()
	require.Nil(t, err)

	_, _, err = grpcClientError.GetChainID()
	require.NotNil(t, err)

	_, _, _, err = grpcClient.GetDirectTransactionMeta("farbic")
	require.Nil(t, err)

	_, _, _, err = grpcClientError.GetDirectTransactionMeta("farbic")
	require.NotNil(t, err)

	require.Panics(t, func() {
		grpcClientError.handleOffChainDataReqStream(nil, nil)
	})
	require.Panics(t, func() {
		grpcClientError.GetOffChainDataReq()
	})

	require.Panics(t, func() {
		grpcClientError.GetUpdateMeta()
	})

	_, err = grpcClient.SubmitIBTPBatch([]string{}, []uint64{}, []string{}, []pb.IBTP_Type{}, []*pb.Content{}, []*pb.BxhProof{}, []bool{})
	require.Nil(t, err)

	_, err = grpcClient.SubmitReceiptBatch([]string{}, []uint64{}, []string{}, []pb.IBTP_Type{}, []*pb.Result{}, []*pb.BxhProof{})
	require.Nil(t, err)
}
