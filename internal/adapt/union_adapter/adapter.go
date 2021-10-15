package union_adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/adapt/bxh_adapter"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/router"
	"github.com/sirupsen/logrus"
)

const maxChSize = 1 << 10

// UnionAdapter represents the necessary data for sync tx from bitxhub
type UnionAdapter struct {
	client rpcx.Client
	logger logrus.FieldLogger
	ibtpC  chan *pb.IBTP

	bxhAdapter bxh_adapter.BxhAdapterI
	peerMgr    peermgr.PeerManager
	router     router.Router
	mode       string
	bxhId      string
	ctx        context.Context
	cancel     context.CancelFunc
}

func (b *UnionAdapter) MonitorUpdatedMeta() chan *[]byte {
	panic("implement me")
}

func (b *UnionAdapter) SendUpdatedMeta(byte []byte) error {
	panic("implement me")
}

func (b *UnionAdapter) GetServiceIDList() ([]string, error) {
	panic("implement me")
}

func (b *UnionAdapter) Name() string {
	return fmt.Sprintf("union:%s", b.bxhId)
}

func New(peerMgr peermgr.PeerManager, bxh bxh_adapter.BxhAdapterI, logger logrus.FieldLogger) (*UnionAdapter, error) {

	router := router.New(peerMgr, loggers.Logger(loggers.Router), peerMgr.(*peermgr.Swarm).ConnectedPeerIDs())
	da := &UnionAdapter{
		logger:     logger,
		peerMgr:    peerMgr,
		router:     router,
		bxhAdapter: bxh,
		ibtpC:      make(chan *pb.IBTP, maxChSize),
		bxhId:      bxh.ID(),
	}

	return da, nil
}

func (u *UnionAdapter) Start() error {
	if u.ibtpC == nil {
		u.ibtpC = make(chan *pb.IBTP, maxChSize)
	}
	if err := u.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_SEND, u.handleRouterSendIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp send handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_GET, u.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp get handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_RECEIPT_GET, u.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp get handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_INTERCHAIN_GET, u.handleRouterInterchain); err != nil {
		return fmt.Errorf("register router interchain handler: %w", err)
	}

	go func() {
		for {
			err := u.router.Broadcast(u.bxhId)
			if err != nil {
				u.logger.Warnf("broadcast BitXHub ID %s: %w", u.bxhId, err)
			}

			time.Sleep(time.Second)
		}
	}()
	u.logger.Info("UnionAdapter started")
	return nil
}

func (b *UnionAdapter) Stop() error {
	err := b.peerMgr.Stop()
	if err != nil {
		return err
	}
	err = b.router.Stop()
	if err != nil {
		return err
	}
	close(b.ibtpC)
	b.ibtpC = nil

	b.cancel()
	b.logger.Info("UnionAdapter stopped")
	return nil
}

func (b *UnionAdapter) SendIBTP(ibtp *pb.IBTP) error {
	entry := b.logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	})
	//ibtp.Proof = signs

	err := b.router.Route(ibtp)
	if err != nil {
		entry.WithField("err", err).Warn("Send union IBTP failed")
		return err
	}
	entry.Info("Send union IBTP successfully")
	return nil
}

func (b *UnionAdapter) MonitorIBTP() chan *pb.IBTP {
	return b.ibtpC
}

func (b *UnionAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	ibtp, err := b.router.QueryIBTP(id, isReq)
	return ibtp, err
}

func (b *UnionAdapter) QueryInterchain(fullServiceId string) (*pb.Interchain, error) {
	bxhId, _, serviceID, err := pb.ParseFullServiceID(fullServiceId)
	if err != nil {
		return nil, err
	}
	interchain, err := b.router.QueryInterchain(bxhId, serviceID)
	return interchain, err
}
