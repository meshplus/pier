package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
)

var _ pb.AppchainPluginServer = (*GRPCServer)(nil)

var _ Client = (*GRPCClient)(nil)

// ---- gRPC Server domain ----
type GRPCServer struct {
	Impl Client
}

func (s *GRPCServer) GetLockEvent(empty *pb.Empty, server pb.AppchainPlugin_GetLockEventServer) error {
	panic("implement me")
}

func (s *GRPCServer) GetUpdateMeta(empty *pb.Empty, server pb.AppchainPlugin_GetUpdateMetaServer) error {
	panic("implement me")
}

func (s *GRPCServer) UnEscrow(ctx context.Context, lock *pb.UnLock) (*pb.Empty, error) {
	panic("implement me")
}

func (s *GRPCServer) QueryFilterLockStart(ctx context.Context, request *pb.QueryFilterLockStartRequest) (*pb.QueryFilterLockStartResponse, error) {
	panic("implement me")
}

func (s *GRPCServer) QueryLockEventByIndex(ctx context.Context, request *pb.QueryLockEventByIndexRequest) (*pb.LockEvent, error) {
	panic("implement me")
}

func (s *GRPCServer) QueryAppchainIndex(ctx context.Context, empty *pb.Empty) (*pb.QueryAppchainIndexResponse, error) {
	panic("implement me")
}

func (s *GRPCServer) QueryRelayIndex(ctx context.Context, empty *pb.Empty) (*pb.QueryRelayIndexResponse, error) {
	panic("implement me")
}

func (s *GRPCServer) Initialize(_ context.Context, req *pb.InitializeRequest) (*pb.Empty, error) {
	err := s.Impl.Initialize(req.ConfigPath, req.Extra)
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
		case ibtp, ok := <-ibtpC:
			if !ok {
				logger.Error("get ibtp channel has closed")
				return nil
			}
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

func (s *GRPCServer) SubmitReceipt(ctx context.Context, ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitReceipt(ibtp)
}

func (s *GRPCServer) GetOutMessage(_ context.Context, req *pb.GetMessageRequest) (*pb.IBTP, error) {
	return s.Impl.GetOutMessage(req.ServicePair, req.Idx)
}

func (s *GRPCServer) GetReceiptMessage(_ context.Context, req *pb.GetMessageRequest) (*pb.IBTP, error) {
	return s.Impl.GetReceiptMessage(req.ServicePair, req.Idx)
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

func (s *GRPCServer) GetDstRollbackMeta(ctx context.Context, empty *pb.Empty) (*pb.GetMetaResponse, error) {
	m, err := s.Impl.GetDstRollbackMeta()
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

func (s *GRPCServer) GetServices(ctx context.Context, empty *pb.Empty) (*pb.ServicesResponse, error) {
	services, err := s.Impl.GetServices()
	if err != nil {
		return nil, err
	}

	return &pb.ServicesResponse{
		Service: services,
	}, nil
}

func (s *GRPCServer) GetChainID(ctx context.Context, empty *pb.Empty) (*pb.ChainIDResponse, error) {
	bxhID, apchainID, err := s.Impl.GetChainID()
	if err != nil {
		return nil, err
	}

	return &pb.ChainIDResponse{
		BxhID:      bxhID,
		AppchainID: apchainID,
	}, nil
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

func (g *GRPCClient) Initialize(configPath string, extra []byte) error {
	_, err := g.client.Initialize(g.doneContext, &pb.InitializeRequest{
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
	g.doneContext.Done()
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

func (g *GRPCClient) SubmitReceipt(ibtp *pb.IBTP) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitReceipt(g.doneContext, ibtp)
		if err != nil {
			logger.Error("submit ibtp receipt to plugin server",
				"ibtp id", ibtp.ID(),
				"err", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second))

	return response, nil
}

func (g *GRPCClient) GetOutMessage(servicePair string, idx uint64) (*pb.IBTP, error) {
	return g.client.GetOutMessage(g.doneContext, &pb.GetMessageRequest{
		ServicePair: servicePair,
		Idx:         idx,
	})
}

func (g *GRPCClient) GetReceiptMessage(servicePair string, idx uint64) (*pb.IBTP, error) {
	if idx != 1 {
		return nil, fmt.Errorf("not found")
	}
	return g.client.GetReceiptMessage(g.doneContext, &pb.GetMessageRequest{
		ServicePair: servicePair,
		Idx:         idx,
	})
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
	response, err := g.client.GetCallbackMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetDstRollbackMeta() (map[string]uint64, error) {
	response, err := g.client.GetDstRollbackMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.Meta, nil
}

func (g *GRPCClient) GetServices() ([]string, error) {
	response, err := g.client.GetServices(g.doneContext, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return response.GetService(), nil
}

func (g *GRPCClient) GetChainID() (string, string, error) {
	response, err := g.client.GetChainID(g.doneContext, &pb.Empty{})
	if err != nil {
		return "", "", err
	}

	return response.BxhID, response.AppchainID, nil
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
