package plugins

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/pkg/plugins/mock_client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	from       = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to         = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50161"
	configPath = "fabric"
	name       = "app"
	ty         = "fabric"
)

func TestCreateClient(t *testing.T) {
	appchainConfig := repo.Appchain{
		Config: "fabric",
		Plugin: "appchain_plugin",
	}

	_, _, err := CreateClient(from, appchainConfig, make([]byte, 0))
	require.NotNil(t, err)
}

func TestGRPCServer_Error(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	grpcServer := GRPCServer{cli}

	cli.EXPECT().GetInMeta().Return(nil, fmt.Errorf("get in meta error")).AnyTimes()
	cli.EXPECT().GetOutMeta().Return(nil, fmt.Errorf("get out meta error")).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(nil, fmt.Errorf("get callback error")).AnyTimes()

	_, err := grpcServer.GetInMeta(ctx, nil)
	require.NotNil(t, err)

	_, err = grpcServer.GetOutMeta(ctx, nil)
	require.NotNil(t, err)

	_, err = grpcServer.GetCallbackMeta(ctx, nil)
	require.NotNil(t, err)

}
func TestGRPCServer(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	grpcServer := GRPCServer{cli}
	initReq := &pb.InitializeRequest{
		ConfigPath: configPath,
		PierId:     from,
		Extra:      make([]byte, 0),
	}
	outReq := &pb.GetOutMessageRequest{
		To:  to,
		Idx: uint64(1),
	}
	inReq := &pb.GetInMessageRequest{
		From: from,
		Idx:  uint64(1),
	}

	ch := make(chan *pb.IBTP, 1)
	close(ch)

	cli.EXPECT().Initialize(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	cli.EXPECT().Start().Return(nil).AnyTimes()
	cli.EXPECT().Stop().Return(nil).AnyTimes()
	cli.EXPECT().GetIBTP().Return(ch).AnyTimes()
	cli.EXPECT().SubmitIBTP(gomock.Any()).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetOutMessage(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetInMessage(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetInMeta().Return(nil, nil).AnyTimes()
	cli.EXPECT().GetOutMeta().Return(nil, nil).AnyTimes()
	cli.EXPECT().GetCallbackMeta().Return(nil, nil).AnyTimes()
	cli.EXPECT().Name().Return(name).AnyTimes()
	cli.EXPECT().Type().Return(ty).AnyTimes()

	_, err := grpcServer.Initialize(ctx, initReq)
	require.Nil(t, err)

	_, err = grpcServer.Start(ctx, nil)
	require.Nil(t, err)

	_, err = grpcServer.Stop(ctx, nil)
	require.Nil(t, err)

	require.Nil(t, grpcServer.GetIBTP(nil, &mockAppchainPlugin_GetIBTPServer{}))

	_, err = grpcServer.SubmitIBTP(ctx, nil)
	require.Nil(t, err)

	_, err = grpcServer.GetOutMessage(ctx, outReq)
	require.Nil(t, err)

	_, err = grpcServer.GetInMessage(ctx, inReq)
	require.Nil(t, err)

	_, err = grpcServer.GetInMeta(ctx, nil)
	require.Nil(t, err)

	_, err = grpcServer.GetOutMeta(ctx, nil)
	require.Nil(t, err)

	_, err = grpcServer.GetCallbackMeta(ctx, nil)
	require.Nil(t, err)

	_, err = grpcServer.CommitCallback(ctx, nil)
	require.Nil(t, err)

	nameRet, err := grpcServer.Name(ctx, nil)
	require.Nil(t, err)
	require.Equal(t, name, nameRet.Name)

	typeRet, err := grpcServer.Type(ctx, nil)
	require.Nil(t, err)
	require.Equal(t, ty, typeRet.Type)
}

func TestGRPCClient(t *testing.T) {
	ctx := context.Background()
	ctx1, _ := context.WithCancel(ctx)
	grpcClient, err := getGRPCClient(ctx, &mockAppchainPluginClient{})
	grpcClientError, err := getGRPCClient(ctx1, &mockAppchainPluginClient{})
	require.Nil(t, err)

	require.Nil(t, grpcClient.Initialize(configPath, from, make([]byte, 0)))
	require.NotNil(t, grpcClient.Initialize(configPath, to, make([]byte, 0)))
	require.Nil(t, grpcClient.Start())
	require.NotNil(t, grpcClientError.Start())
	require.Nil(t, grpcClient.Stop())
	require.NotNil(t, grpcClientError.Stop())
	require.Equal(t, 0, len(grpcClient.GetIBTP()))
	require.Panics(t, func() {
		grpcClientError.GetIBTP()
	})

	_, err = grpcClient.SubmitIBTP(nil)
	require.Nil(t, err)

	ibtp, err := grpcClient.GetOutMessage(to, uint64(1))
	require.Nil(t, err)
	require.Equal(t, to, ibtp.To)
	require.Equal(t, uint64(1), ibtp.Index)

	_, err = grpcClient.GetInMessage(from, uint64(1))
	require.Nil(t, err)
	_, err = grpcClient.GetInMessage(from, uint64(2))
	require.NotNil(t, err)

	_, err = grpcClient.GetInMeta()
	require.Nil(t, err)
	_, err = grpcClientError.GetInMeta()
	require.NotNil(t, err)

	_, err = grpcClient.GetOutMeta()
	require.Nil(t, err)
	_, err = grpcClientError.GetOutMeta()
	require.NotNil(t, err)

	_, err = grpcClient.GetCallbackMeta()
	require.Nil(t, err)
	_, err = grpcClientError.GetCallbackMeta()
	require.NotNil(t, err)

	require.Nil(t, grpcClient.CommitCallback(nil))

	require.Equal(t, name, grpcClient.Name())
	require.Equal(t, "", grpcClientError.Name())

	require.Equal(t, ty, grpcClient.Type())
	require.Equal(t, "", grpcClientError.Type())
}

func TestAppchainGRPCPlugin_GRPCClient(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	grpcPlugin := AppchainGRPCPlugin{
		Impl: cli,
	}

	_, err := grpcPlugin.GRPCClient(ctx, nil, nil)
	require.Nil(t, err)
}

func TestAppchainGRPCPlugin_GRPCServer(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	cli := mock_client.NewMockClient(mockCtl)
	grpcPlugin := AppchainGRPCPlugin{
		Impl: cli,
	}

	require.Panics(t, func() {
		grpcPlugin.GRPCServer(nil, nil)
	})
}

func getGRPCClient(ctx context.Context, mc *mockAppchainPluginClient) (*GRPCClient, error) {
	return &GRPCClient{
		client:      mc,
		doneContext: ctx,
	}, nil
}

//==========================================================
type mockAppchainPluginClient struct {
}

func (mc *mockAppchainPluginClient) Initialize(ctx context.Context, in *pb.InitializeRequest, opts ...grpc.CallOption) (*pb.Empty, error) {
	if in.PierId == to {
		return nil, fmt.Errorf("initialize error")
	}
	return nil, nil
}

func (mc *mockAppchainPluginClient) Start(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Empty, error) {
	if ctx == context.Background() {
		return nil, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient start error")
}

func (mc *mockAppchainPluginClient) Stop(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Empty, error) {
	if ctx == context.Background() {
		return nil, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient stop error")
}

func (mc *mockAppchainPluginClient) GetIBTP(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (pb.AppchainPlugin_GetIBTPClient, error) {
	if ctx == context.Background() {
		return &mockAppchainPlugin_GetIBTPClient{}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient get ibtp error")
}

func (mc *mockAppchainPluginClient) SubmitIBTP(ctx context.Context, in *pb.IBTP, opts ...grpc.CallOption) (*pb.SubmitIBTPResponse, error) {
	return nil, nil
}

func (mc *mockAppchainPluginClient) GetOutMessage(ctx context.Context, in *pb.GetOutMessageRequest, opts ...grpc.CallOption) (*pb.IBTP, error) {
	return &pb.IBTP{
		To:        in.To,
		Index:     in.Idx,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

func (mc *mockAppchainPluginClient) GetInMessage(ctx context.Context, in *pb.GetInMessageRequest, opts ...grpc.CallOption) (*pb.GetInMessageResponse, error) {
	if in.Idx == uint64(1) {
		return &pb.GetInMessageResponse{
			Result: make([][]byte, 0),
		}, nil
	} else {
		return nil, fmt.Errorf("get in message error")
	}
}

func (mc *mockAppchainPluginClient) GetInMeta(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.GetMetaResponse, error) {
	if ctx == context.Background() {
		return &pb.GetMetaResponse{
			Meta: make(map[string]uint64, 0),
		}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient get in meta error")
}

func (mc *mockAppchainPluginClient) GetOutMeta(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.GetMetaResponse, error) {
	if ctx == context.Background() {
		return &pb.GetMetaResponse{
			Meta: nil,
		}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient get out meta error")
}

func (mc *mockAppchainPluginClient) GetCallbackMeta(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.GetMetaResponse, error) {
	if ctx == context.Background() {
		return &pb.GetMetaResponse{
			Meta: make(map[string]uint64, 0),
		}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient get callback meta error")
}

func (mc *mockAppchainPluginClient) CommitCallback(ctx context.Context, in *pb.IBTP, opts ...grpc.CallOption) (*pb.Empty, error) {
	return nil, nil
}

func (mc *mockAppchainPluginClient) GetReceipt(ctx context.Context, in *pb.IBTP, opts ...grpc.CallOption) (*pb.IBTP, error) {
	return &pb.IBTP{
		From:      in.From,
		To:        in.To,
		Index:     in.Index,
		Timestamp: time.Now().UnixNano(),
		Type:      pb.IBTP_RECEIPT_SUCCESS,
	}, nil
}

func (mc *mockAppchainPluginClient) Name(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.NameResponse, error) {
	if ctx == context.Background() {
		return &pb.NameResponse{
			Name: name,
		}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient name error")
}

func (mc *mockAppchainPluginClient) Type(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.TypeResponse, error) {
	if ctx == context.Background() {
		return &pb.TypeResponse{
			Type: ty,
		}, nil
	}
	return nil, fmt.Errorf("mockAppchainPluginClient type error")
}

var _ pb.AppchainPluginClient = &mockAppchainPluginClient{}

//====================================
type mockAppchainPlugin_GetIBTPClient struct {
}

func (m mockAppchainPlugin_GetIBTPClient) Recv() (*pb.IBTP, error) {
	return nil, nil
}

func (m mockAppchainPlugin_GetIBTPClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m mockAppchainPlugin_GetIBTPClient) Trailer() metadata.MD {
	return nil
}

func (m mockAppchainPlugin_GetIBTPClient) CloseSend() error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPClient) Context() context.Context {
	ctx := context.Background()
	return ctx
}

func (m mockAppchainPlugin_GetIBTPClient) SendMsg(interface{}) error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPClient) RecvMsg(interface{}) error {
	return nil
}

var _ pb.AppchainPlugin_GetIBTPClient = &mockAppchainPlugin_GetIBTPClient{}

//==================================
type mockAppchainPlugin_GetIBTPServer struct {
}

func (m mockAppchainPlugin_GetIBTPServer) Send(ibtp *pb.IBTP) error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPServer) SetHeader(md metadata.MD) error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPServer) SendHeader(md metadata.MD) error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPServer) SetTrailer(md metadata.MD) {
	return
}

func (m mockAppchainPlugin_GetIBTPServer) Context() context.Context {
	ctx := context.Background()
	return ctx
}

func (m mockAppchainPlugin_GetIBTPServer) SendMsg(interface{}) error {
	return nil
}

func (m mockAppchainPlugin_GetIBTPServer) RecvMsg(interface{}) error {
	return nil
}

var _ pb.AppchainPlugin_GetIBTPServer = &mockAppchainPlugin_GetIBTPServer{}
