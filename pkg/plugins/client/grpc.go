package client

import (
	"context"

	"github.com/meshplus/bitxhub-model/pb"
)

// ---- gRPC Server domain ----
type GRPCServer struct {
	Impl Client
}

func (s *GRPCServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.Empty, error) {
	err := s.Impl.Initialize(req.ConfigPath, req.PierId, req.Extra)
	return &pb.Empty{}, err
}

func (s *GRPCServer) Start(context.Context, *pb.Empty) (*pb.Empty, error) {
	err := s.Impl.Start()
	return &pb.Empty{}, err
}

func (s *GRPCServer) Stop(context.Context, *pb.Empty) (*pb.Empty, error) {
	err := s.Impl.Stop()
	return &pb.Empty{}, err
}

func (s *GRPCServer) GetIBTP(in *pb.Empty, conn pb.AppchainPlugin_GetIBTPServer) error {
	ibtpC := s.Impl.GetIBTP()

	var ibtp *pb.IBTP
	go func() error {
		for {
			ibtp = <-ibtpC
			err := conn.Send(ibtp)
			if err != nil {
				return err
			}
		}
	}()

	return nil
}

func (s *GRPCServer) SubmitIBTP(ctx context.Context, ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitIBTP(ibtp)
}

func (s *GRPCServer) GetOutMessage(ctx context.Context, req *pb.GetOutMessageRequest) (*pb.IBTP, error) {
	return s.Impl.GetOutMessage(req.To, req.Idx)
}

func (s *GRPCServer) GetInMessage(ctx context.Context, req *pb.GetInMessageRequest) (*pb.GetInMessageResponse, error) {
	reslut, err := s.Impl.GetInMessage(req.From, req.Idx)

	return &pb.GetInMessageResponse{
		Result: reslut,
	}, err
}

func (s *GRPCServer) GetInMeta(context.Context, *pb.Empty) (*pb.GetMetaResponse, error) {
	m, err := s.Impl.GetInMeta()
	if err != nil {
		return nil, err
	}

	return &pb.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) GetOutMeta(context.Context, *pb.Empty) (*pb.GetMetaResponse, error) {
	m, err := s.Impl.GetOutMeta()
	if err != nil {
		return nil, err
	}

	return &pb.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) GetCallbackMeta(context.Context, *pb.Empty) (*pb.GetMetaResponse, error) {
	m, err := s.Impl.GetCallbackMeta()
	if err != nil {
		return nil, err
	}

	return &pb.GetMetaResponse{
		Meta: m,
	}, nil
}

func (s *GRPCServer) CommitCallback(ctx context.Context, ibtp *pb.IBTP) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (s *GRPCServer) Name(context.Context, *pb.Empty) (*pb.NameResponse, error) {
	return &pb.NameResponse{
		Name: s.Impl.Name(),
	}, nil
}

func (s *GRPCServer) Type(context.Context, *pb.Empty) (*pb.TypeResponse, error) {
	return &pb.TypeResponse{
		Type: s.Impl.Type(),
	}, nil
}

// ---- gRPC Client domain ----

// GRPCClient is an implementation of Client that talks over RPC.
type GRPCClient struct {
	client      pb.AppchainPluginClient
	doneContect context.Context
}

func (g *GRPCClient) Initialize(configPath string, pierID string, extra []byte) error {
	_, err := g.client.Initialize(g.doneContect, &pb.InitializeRequest{
		PierId:     pierID,
		ConfigPath: configPath,
		Extra:      extra,
	})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) Start() error {
	_, err := g.client.Start(g.doneContect, &pb.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) Stop() error {
	_, err := g.client.Stop(g.doneContect, &pb.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) GetIBTP() chan *pb.IBTP {
	conn, err := g.client.GetIBTP(g.doneContect, &pb.Empty{})
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

func (g *GRPCClient) SubmitIBTP(ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	response, err := g.client.SubmitIBTP(g.doneContect, ibtp)
	if err != nil {
		return &pb.SubmitIBTPResponse{}, err
	}

	return response, nil
}

func (g *GRPCClient) GetOutMessage(to string, idx uint64) (*pb.IBTP, error) {
	return g.client.GetOutMessage(g.doneContect, &pb.GetOutMessageRequest{
		To:  to,
		Idx: idx,
	})
}

func (g *GRPCClient) GetInMessage(from string, idx uint64) ([][]byte, error) {
	response, err := g.client.GetInMessage(g.doneContect, &pb.GetInMessageRequest{
		From: from,
		Idx:  idx,
	})
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

func (g *GRPCClient) GetInMeta() (map[string]uint64, error) {
	response, err := g.client.GetInMeta(g.doneContect, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetOutMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContect, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetCallbackMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContect, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) CommitCallback(ibtp *pb.IBTP) error {
	return nil
}

func (g *GRPCClient) Name() string {
	response, err := g.client.Name(g.doneContect, &pb.Empty{})
	if err != nil {
		return ""
	}

	return response.Name
}

func (g *GRPCClient) Type() string {
	response, err := g.client.Type(g.doneContect, &pb.Empty{})
	if err != nil {
		return ""
	}

	return response.Type
}
