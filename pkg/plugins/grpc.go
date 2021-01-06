package plugins

import (
	"context"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
)

// ---- gRPC Server domain ----
type GRPCServer struct {
	Impl Client
}

func (s *GRPCServer) Initialize(_ context.Context, req *pb.InitializeRequest) (*pb.Empty, error) {
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

func (s *GRPCServer) GetIBTP(_ *pb.Empty, conn pb.AppchainPlugin_GetIBTPServer) error {
	ctx := conn.Context()
	ibtpC := s.Impl.GetIBTP()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ibtp := <-ibtpC:
			if err := retry.Retry(func(attempt uint) error {
				if err := conn.Send(ibtp); err != nil {
					logger.Error("plugin send ibtp err", "error", err)
					return err
				}
				return nil
			}, strategy.Wait(1*time.Second)); err != nil {
				logger.Error("Execution of plugin server sending ibtp failed", "error", err)
			}
		}
	}
}

func (s *GRPCServer) SubmitIBTP(_ context.Context, ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitIBTP(ibtp)
}

func (s *GRPCServer) GetOutMessage(_ context.Context, req *pb.GetOutMessageRequest) (*pb.IBTP, error) {
	return s.Impl.GetOutMessage(req.To, req.Idx)
}

func (s *GRPCServer) GetInMessage(_ context.Context, req *pb.GetInMessageRequest) (*pb.GetInMessageResponse, error) {
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

func (s *GRPCServer) CommitCallback(_ context.Context, _ *pb.IBTP) (*pb.Empty, error) {
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
	doneContext context.Context
}

func (g *GRPCClient) Initialize(configPath string, pierID string, extra []byte) error {
	_, err := g.client.Initialize(g.doneContext, &pb.InitializeRequest{
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
	_, err := g.client.Start(g.doneContext, &pb.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) Stop() error {
	_, err := g.client.Stop(g.doneContext, &pb.Empty{})
	if err != nil {
		return err
	}
	return nil
}

func (g *GRPCClient) GetIBTP() chan *pb.IBTP {
	conn, err := g.client.GetIBTP(g.doneContext, &pb.Empty{})
	if err != nil {
		// todo: more concise panic info
		panic(err.Error())
	}

	ibtpQ := make(chan *pb.IBTP, 10)
	go g.handleIBTPStream(conn, ibtpQ)

	return ibtpQ
}

func (g *GRPCClient) handleIBTPStream(conn pb.AppchainPlugin_GetIBTPClient, ibtpQ chan *pb.IBTP) {
	defer close(ibtpQ)

	for {
		var (
			ibtp *pb.IBTP
			err  error
		)
		if err := retry.Retry(func(attempt uint) error {
			ibtp, err = conn.Recv()
			if err != nil {
				logger.Error("Plugin grpc client recv ibtp", "error", err)
				// End the stream
				return err
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Error("Execution of client recv failed", "error", err)
		}

		select {
		case <-g.doneContext.Done():
			return
		case ibtpQ <- ibtp:
		}
	}
}

func (g *GRPCClient) SubmitIBTP(ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitIBTP(g.doneContext, ibtp)
		if err != nil {
			logger.Error("submit ibtp to plugin server",
				"ibtp id", ibtp.ID(),
				"err", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second))

	return response, nil
}

func (g *GRPCClient) GetOutMessage(to string, idx uint64) (*pb.IBTP, error) {
	return g.client.GetOutMessage(g.doneContext, &pb.GetOutMessageRequest{
		To:  to,
		Idx: idx,
	})
}

func (g *GRPCClient) GetInMessage(from string, idx uint64) ([][]byte, error) {
	response, err := g.client.GetInMessage(g.doneContext, &pb.GetInMessageRequest{
		From: from,
		Idx:  idx,
	})
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

func (g *GRPCClient) GetInMeta() (map[string]uint64, error) {
	response, err := g.client.GetInMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetOutMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	if response.Meta == nil {
		response.Meta = make(map[string]uint64)
	}
	return response.Meta, nil
}

func (g *GRPCClient) GetCallbackMeta() (map[string]uint64, error) {
	response, err := g.client.GetOutMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) CommitCallback(_ *pb.IBTP) error {
	return nil
}

func (g *GRPCClient) Name() string {
	response, err := g.client.Name(g.doneContext, &pb.Empty{})
	if err != nil {
		return ""
	}

	return response.Name
}

func (g *GRPCClient) Type() string {
	response, err := g.client.Type(g.doneContext, &pb.Empty{})
	if err != nil {
		return ""
	}

	return response.Type
}
