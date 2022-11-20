package union_adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/router"
	"github.com/sirupsen/logrus"
)

const maxChSize = 1 << 10

var _ adapt.Adapt = (*UnionAdapter)(nil)

// UnionAdapter represents the necessary data for sync tx from bitxhub
type UnionAdapter struct {
	client rpcx.Client
	logger logrus.FieldLogger
	ibtpC  chan *pb.IBTP

	bxhAdapter adapt.Adapt
	peerMgr    peermgr.PeerManager
	router     router.Router
	bxhId      string
	ctx        context.Context
	cancel     context.CancelFunc
}

func (u *UnionAdapter) InitIbtpPool(_, _ string, _ pb.IBTP_Category, _ uint64) {
	return
}

func (u *UnionAdapter) MonitorUpdatedMeta() chan *[]byte {
	return nil
}

func (u *UnionAdapter) SendUpdatedMeta(_ []byte) error {
	return nil
}

func (u *UnionAdapter) GetServiceIDList() ([]string, error) {
	return nil, nil
}

func (u *UnionAdapter) Name() string {
	return fmt.Sprintf("union:%s", u.bxhId)
}

func (u *UnionAdapter) ID() string {
	return fmt.Sprintf("%s", u.bxhId)
}

func New(peerMgr peermgr.PeerManager, bxh adapt.Adapt, logger logrus.FieldLogger) (*UnionAdapter, error) {
	router := router.New(peerMgr, loggers.Logger(loggers.Router))
	ctx, cancel := context.WithCancel(context.Background())
	da := &UnionAdapter{
		logger:     logger,
		peerMgr:    peerMgr,
		router:     router,
		bxhAdapter: bxh,
		ibtpC:      make(chan *pb.IBTP, maxChSize),
		bxhId:      bxh.ID(),
		ctx:        ctx,
		cancel:     cancel,
	}

	return da, nil
}

func (u *UnionAdapter) Start() error {
	if u.ibtpC == nil {
		u.ibtpC = make(chan *pb.IBTP, maxChSize)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_SEND, u.handleRouterSendIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp send handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_GET, u.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp get handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ADDRESS_GET, u.handleGetAddressMessage); err != nil {
		return fmt.Errorf("register get address msg handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_IBTP_RECEIPT_GET, u.handleRouterGetIBTPMessage); err != nil {
		return fmt.Errorf("register router ibtp receipt get handler: %w", err)
	}

	if err := u.peerMgr.RegisterMsgHandler(pb.Message_ROUTER_INTERCHAIN_GET, u.handleRouterInterchain); err != nil {
		return fmt.Errorf("register router interchain handler: %w", err)
	}

	if err := u.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}

	go func() {
		for {
			select {
			case <-u.ctx.Done():
				u.logger.Info("UnionAdapter Broadcast Stoped!")
				return
			default:
				err := u.router.Broadcast(u.bxhId)
				if err != nil {
					u.logger.Warnf("broadcast BitXHub ID %s: %w", u.bxhId, err)
				}
				time.Sleep(time.Second)
			}
		}
	}()
	u.logger.Info("UnionAdapter started")
	return nil
}

func (u *UnionAdapter) Stop() error {
	err := u.peerMgr.Stop()
	if err != nil {
		return err
	}
	err = u.router.Stop()
	if err != nil {
		return err
	}
	close(u.ibtpC)
	u.ibtpC = nil

	u.cancel()
	u.logger.Info("UnionAdapter stopped")
	return nil
}

func (u *UnionAdapter) SendIBTP(ibtp *pb.IBTP) error {
	entry := u.logger.WithFields(logrus.Fields{
		"type": ibtp.Type,
		"id":   ibtp.ID(),
	})
	// todo get multiSigns from bxhAdapt
	//ibtp.Proof = signs

	err := u.router.Route(ibtp)
	if err != nil {
		entry.WithField("err", err).Warn("Send union IBTP failed")
		return err
	}
	entry.Info("Send union IBTP successfully")
	return nil
}

func (u *UnionAdapter) MonitorIBTP() chan *pb.IBTP {
	return u.ibtpC
}

func (u *UnionAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	ibtp, err := u.router.QueryIBTP(id, isReq)
	return ibtp, err
}

func (u *UnionAdapter) QueryInterchain(fullServiceId string) (*pb.Interchain, error) {
	bxhId, _, _, err := pb.ParseFullServiceID(fullServiceId)
	if err != nil {
		return nil, err
	}
	interchain, err := u.router.QueryInterchain(bxhId, fullServiceId)
	return interchain, err
}
