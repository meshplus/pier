package plugins

import (
	"context"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-model/pb"
)

var _ pb.AppchainPluginServer = (*GRPCServer)(nil)

var _ agency.Client = (*GRPCClient)(nil)

// ---- gRPC Server domain ----
type GRPCServer struct {
	Impl agency.Client
}

func (s *GRPCServer) GetUpdateMeta(empty *pb.Empty, server pb.AppchainPlugin_GetUpdateMetaServer) error {
	panic("implement me")
}

func (s *GRPCServer) Initialize(_ context.Context, req *pb.InitializeRequest) (*pb.Empty, error) {
	err := s.Impl.Initialize(req.ConfigPath, req.Extra, req.Mode)
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

func (s *GRPCServer) GetIBTPCh(_ *pb.Empty, conn pb.AppchainPlugin_GetIBTPChServer) error {
	ctx := conn.Context()
	ibtpC := s.Impl.GetIBTPCh()

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

func (s *GRPCServer) GetOffChainData(ctx context.Context, req *pb.GetDataRequest) (*pb.OffChainDataInfo, error) {
	return s.Impl.GetOffChainData(req)
}

func (s *GRPCServer) GetOffChainDataReq(_ *pb.Empty, conn pb.AppchainPlugin_GetOffChainDataReqServer) error {
	ctx := conn.Context()
	c := s.Impl.GetOffChainDataReq()

	for {
		select {
		case <-ctx.Done():
			return nil
		case req, ok := <-c:
			if !ok {
				logger.Error("get data request channel has closed")
				return nil
			}
			if err := retry.Retry(func(attempt uint) error {
				if err := conn.Send(req); err != nil {
					logger.Error("plugin send data req err", "error", err)
					return err
				}
				return nil
			}, strategy.Wait(1*time.Second)); err != nil {
				logger.Error("Execution of plugin server sending data request failed", "error", err)
			}
		}
	}
}

func (s *GRPCServer) SubmitOffChainData(_ context.Context, res *pb.GetDataResponse) (*pb.Empty, error) {
	return &pb.Empty{}, s.Impl.SubmitOffChainData(res)
}

func (s *GRPCServer) SubmitIBTP(_ context.Context, ibtp *pb.SubmitIBTPRequest) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitIBTP(ibtp.From, ibtp.Index, ibtp.ServiceId, ibtp.Type, ibtp.Content, ibtp.BxhProof, ibtp.IsEncrypted)
}

func (s *GRPCServer) SubmitIBTPBatch(_ context.Context, ibtpBatch *pb.SubmitIBTPRequestBatch) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitIBTPBatch(ibtpBatch.From, ibtpBatch.Index, ibtpBatch.ServiceId, ibtpBatch.Type, ibtpBatch.Content, ibtpBatch.BxhProof, ibtpBatch.IsEncrypted)
}

func (s *GRPCServer) SubmitReceipt(ctx context.Context, ibtp *pb.SubmitReceiptRequest) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitReceipt(ibtp.To, ibtp.Index, ibtp.ServiceId, ibtp.Type, ibtp.Result, ibtp.BxhProof)
}

func (s *GRPCServer) SubmitReceiptBatch(ctx context.Context, ibtpBatch *pb.SubmitReceiptRequestBatch) (*pb.SubmitIBTPResponse, error) {
	return s.Impl.SubmitReceiptBatch(ibtpBatch.To, ibtpBatch.Index, ibtpBatch.ServiceId, ibtpBatch.Type, ibtpBatch.Result, ibtpBatch.BxhProof)
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

func (s *GRPCServer) GetAppchainInfo(ctx context.Context, request *pb.ChainInfoRequest) (*pb.ChainInfoResponse, error) {
	broker, trustedRoot, ruleAddr, err := s.Impl.GetAppchainInfo(request.ChainID)
	if err != nil {
		return nil, err
	}

	return &pb.ChainInfoResponse{
		Broker:      broker,
		TrustedRoot: trustedRoot,
		RuleAddr:    ruleAddr,
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

func (s *GRPCServer) GetDirectTransactionMeta(ctx context.Context, request *pb.DirectTransactionMetaRequest) (*pb.DirectTransactionMetaResponse, error) {
	startTimestamp, timeoutPeriod, transactionStatus, err := s.Impl.GetDirectTransactionMeta(request.IBTPid)
	if err != nil {
		return nil, err
	}

	return &pb.DirectTransactionMetaResponse{
		StartTimestamp:    startTimestamp,
		TimeoutPeriod:     timeoutPeriod,
		TransactionStatus: transactionStatus,
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

func (g *GRPCClient) Initialize(configPath string, extra []byte, mode string) error {
	_, err := g.client.Initialize(g.doneContext, &pb.InitializeRequest{
		ConfigPath: configPath,
		Extra:      extra,
		Mode:       mode,
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

func (g *GRPCClient) GetIBTPCh() chan *pb.IBTP {
	conn, err := g.client.GetIBTPCh(g.doneContext, &pb.Empty{})
	if err != nil {
		// todo: more concise panic info
		panic(err.Error())
	}

	ibtpQ := make(chan *pb.IBTP, 10)
	go g.handleIBTPStream(conn, ibtpQ)

	return ibtpQ
}

func (g *GRPCClient) GetOffChainData(request *pb.GetDataRequest) (*pb.OffChainDataInfo, error) {
	return g.client.GetOffChainData(g.doneContext, request)
}

func (g *GRPCClient) GetOffChainDataReq() chan *pb.GetDataRequest {
	conn, err := g.client.GetOffChainDataReq(g.doneContext, &pb.Empty{})
	if err != nil {
		panic(err)
	}
	dataReq := make(chan *pb.GetDataRequest)
	go g.handleOffChainDataReqStream(conn, dataReq)
	return dataReq
}

func (g *GRPCClient) SubmitOffChainData(response *pb.GetDataResponse) error {
	logger.Info("start submit offChain data")
	_, err := g.client.SubmitOffChainData(g.doneContext, response)
	return err
}

func (g *GRPCClient) GetUpdateMeta() chan *pb.UpdateMeta {
	conn, err := g.client.GetUpdateMeta(g.doneContext, &pb.Empty{})
	if err != nil {
		// todo: more concise panic info
		panic(err.Error())
	}

	metaQ := make(chan *pb.UpdateMeta, 10)
	go func() {
		defer close(metaQ)
		for {
			var (
				meta *pb.UpdateMeta
				err  error
			)
			if err := retry.Retry(func(attempt uint) error {
				meta, err = conn.Recv()
				if err != nil {
					logger.Error("Plugin grpc client recv meta", "error", err)
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
			case metaQ <- meta:
			}
		}
	}()

	return metaQ
}

func (g *GRPCClient) handleIBTPStream(conn pb.AppchainPlugin_GetIBTPChClient, ibtpQ chan *pb.IBTP) {
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

func (g *GRPCClient) handleOffChainDataReqStream(conn pb.AppchainPlugin_GetOffChainDataReqClient, dataReq chan *pb.GetDataRequest) {
	defer close(dataReq)

	for {
		var (
			req *pb.GetDataRequest
			err error
		)
		if err := retry.Retry(func(attempt uint) error {
			req, err = conn.Recv()
			if err != nil {
				logger.Error("Plugin grpc client recv data request", "error", err)
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
		case dataReq <- req:
			logger.Info("handleDataReqStream req", "req", req)
		}
	}
}

func (g *GRPCClient) SubmitIBTP(from string, index uint64, serviceID string, ibtpType pb.IBTP_Type, content *pb.Content, proof *pb.BxhProof, isEncrypted bool) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitIBTP(g.doneContext, &pb.SubmitIBTPRequest{
			From:        from,
			Index:       index,
			ServiceId:   serviceID,
			Type:        ibtpType,
			Content:     content,
			BxhProof:    proof,
			IsEncrypted: isEncrypted,
		})
		if err != nil {
			logger.Error("submit ibtp to plugin server",
				"err", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second))

	return response, nil
}

func (g *GRPCClient) SubmitIBTPBatch(from []string, index []uint64, serviceID []string, ibtpType []pb.IBTP_Type, content []*pb.Content, proof []*pb.BxhProof, isEncrypted []bool) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitIBTPBatch(g.doneContext, &pb.SubmitIBTPRequestBatch{
			From:        from,
			Index:       index,
			ServiceId:   serviceID,
			Type:        ibtpType,
			Content:     content,
			BxhProof:    proof,
			IsEncrypted: isEncrypted,
		})
		if err != nil {
			logger.Error("submit ibtp to plugin server",
				"err", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second))

	return response, nil
}

func (g *GRPCClient) SubmitReceipt(to string, index uint64, serviceID string, ibtpType pb.IBTP_Type, result *pb.Result, proof *pb.BxhProof) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitReceipt(g.doneContext, &pb.SubmitReceiptRequest{
			To:        to,
			Index:     index,
			ServiceId: serviceID,
			Type:      ibtpType,
			Result:    result,
			BxhProof:  proof,
		})
		if err != nil {
			logger.Error("submit ibtp receipt to plugin server",
				"err", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second))

	return response, nil
}

func (g *GRPCClient) SubmitReceiptBatch(to []string, index []uint64, serviceID []string, ibtpType []pb.IBTP_Type, result []*pb.Result, proof []*pb.BxhProof) (*pb.SubmitIBTPResponse, error) {
	var err error
	response := &pb.SubmitIBTPResponse{}
	retry.Retry(func(attempt uint) error {
		response, err = g.client.SubmitReceiptBatch(g.doneContext, &pb.SubmitReceiptRequestBatch{
			To:        to,
			Index:     index,
			ServiceId: serviceID,
			Type:      ibtpType,
			Result:    result,
			BxhProof:  proof,
		})
		if err != nil {
			logger.Error("submit ibtp receipt to plugin server",
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

func (g *GRPCClient) GetAppchainInfo(chainID string) (string, []byte, string, error) {
	response, err := g.client.GetAppchainInfo(g.doneContext, &pb.ChainInfoRequest{ChainID: chainID})
	if err != nil {
		return "", nil, "", err
	}

	return response.Broker, response.TrustedRoot, response.RuleAddr, nil
}

func (g *GRPCClient) GetChainID() (string, string, error) {
	response, err := g.client.GetChainID(g.doneContext, &pb.Empty{})
	if err != nil {
		return "", "", err
	}

	return response.BxhID, response.AppchainID, nil
}

func (g *GRPCClient) GetDirectTransactionMeta(id string) (uint64, uint64, uint64, error) {
	response, err := g.client.GetDirectTransactionMeta(g.doneContext, &pb.DirectTransactionMetaRequest{IBTPid: id})
	if err != nil {
		return 0, 0, 0, err
	}

	return response.StartTimestamp, response.TimeoutPeriod, response.TransactionStatus, nil
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
