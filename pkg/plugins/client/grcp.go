package client

import (
	"context"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/plugins/proto"
)

// ---- gRPC Server domain ----
type GRPCServer struct {
	Impl Client
}

func (s *GRPCServer) Initialize(ctx context.Context, req *proto.InitializeRequest) (*proto.Empty, error) {
	err := s.Impl.Initialize(req.ConfigPath, req.PierID, req.Extra)
	return &proto.Empty{}, err
}

func (s *GRPCServer) Start(context.Context, *proto.Empty) (*proto.Empty, error) {
	err := s.Impl.Start()
	return &proto.Empty{}, err
}

func (s *GRPCServer) Stop(context.Context, *proto.Empty) (*proto.Empty, error) {
	err := s.Impl.Stop()
	return &proto.Empty{}, err
}

func (s *GRPCServer) GetIBTP(in *proto.Empty, conn proto.AppchainPlugin_GetIBTPServer) error {
	ibtpC := s.Impl.GetIBTP()

	var ibtp *pb.IBTP
	go func() error{
		for {
			ibtp = <- ibtpC
			err := conn.Send(ibtp)
			if err != nil {
				return err
			}
		}
	}()

	return nil
}

func (s *GRPCServer) SubmitIBTP(ctx context.Context, ibtp *pb.IBTP) (*proto.SubmitIBTPResponse, error) {
	return s.Impl.SubmitIBTP(ibtp)
}

func (s *GRPCServer) GetOutMessage(ctx context.Context, req *proto.GetOutMessageRequest) (*pb.IBTP, error) {
	return s.Impl.GetOutMessage(req.To, req.Idx)
}

func (s *GRPCServer) GetInMessage(ctx context.Context, req *proto.GetInMessageRequest) (*proto.GetInMessageResponse, error) {
	reslut, err := s.Impl.GetInMessage(req.From, req.Idx)

	return &proto.GetInMessageResponse{
		Result: reslut,
	}, err
}

func (s *GRPCServer) GetInMeta(context.Context, *proto.Empty) (*proto.GetMetaResponse, error) {
	m, err := s.Impl.GetInMeta()
	if err != nil {
		return nil, err
	}

	return &proto.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) GetOutMeta(context.Context, *proto.Empty) (*proto.GetMetaResponse, error) {
	m, err := s.Impl.GetOutMeta()
	if err != nil {
		return nil, err
	}

	return &proto.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) GetCallbackMeta(context.Context, *proto.Empty) (*proto.GetMetaResponse, error) {
	m, err := s.Impl.GetCallbackMeta()
	if err != nil {
		return nil, err
	}

	return &proto.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) CommitCallback(ctx context.Context, ibtp *pb.IBTP) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *GRPCServer) Name(context.Context, *proto.Empty) (*proto.NameResponse, error) {
	return &proto.NameResponse{
		Name: s.Impl.Name(),
	}, nil
}

func (s *GRPCServer) Type(context.Context, *proto.Empty) (*proto.TypeResponse, error) {
	return &proto.TypeResponse{
		Type: s.Impl.Type(),
	}, nil
}

// ---- gRPC Client domain ----

// GRPCClient is an implementation of Client that talks over RPC.
type GRPCClient struct {
	client      proto.AppchainPluginClient
	doneContect context.Context
}

func (g *GRPCClient) Initialize(configPath string, pierID string, extra []byte) error {
	_, err := g.client.Initialize(g.doneContect, &proto.InitializeRequest{
		PierID:     pierID,
		ConfigPath: configPath,
		Extra:      extra,
	})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) Start() error {
	_, err := g.client.Start(g.doneContect, &proto.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) Stop() error {
	_, err := g.client.Stop(g.doneContect, &proto.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) GetIBTP() chan *pb.IBTP {
	conn, err := g.client.GetIBTP(g.doneContect, &proto.Empty{})
	if err != nil {
		// todo: more concise panic info
		panic(err.Error())
	}

	var ibtpQ chan *pb.IBTP
	go func() {
		for {
			select {
			case <-g.doneContect.Done():
				return
			default:
				ibtp, err := conn.Recv()
				if err != nil {
					return
				}
				ibtpQ <- ibtp
			}
		}
	}()

	return ibtpQ
}

func (g *GRPCClient) SubmitIBTP(ibtp *pb.IBTP) (*proto.SubmitIBTPResponse, error) {
	response, err := g.client.SubmitIBTP(g.doneContect, ibtp)
	if err != nil {
		return &proto.SubmitIBTPResponse{}, err
	}

	return response, nil
}

func (g *GRPCClient) GetOutMessage(to string, idx uint64) (*pb.IBTP, error) {
	return g.client.GetOutMessage(g.doneContect, &proto.GetOutMessageRequest{
		To:  to,
		Idx: idx,
	})
}

func (g *GRPCClient) GetInMessage(from string, idx uint64) ([][]byte, error) {
	response, err := g.client.GetInMessage(g.doneContect, &proto.GetInMessageRequest{
		From: from,
		Idx:  idx,
	})
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

func (g *GRPCClient) GetInMeta() (map[string]uint64, error) {
	response, err := g.client.GetInMeta(g.doneContect, &proto.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetOutMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContect, &proto.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetCallbackMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContect, &proto.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) CommitCallback(ibtp *pb.IBTP) error {
	return nil
}

func (g *GRPCClient) Name() string {
	response, err := g.client.Name(g.doneContect, &proto.Empty{})
	if err != nil {
		return ""
	}

	return response.Name
}

func (g *GRPCClient) Type() string {
	response, err := g.client.Type(g.doneContect, &proto.Empty{})
	if err != nil {
		return ""
	}

	return response.Type
}
