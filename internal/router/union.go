package router

import (
	"context"
	"fmt"

	peerproto "github.com/meshplus/pier/internal/peermgr/proto"

	"github.com/meshplus/bitxhub-kit/log"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("union_router")
var _ Router = (*UnionRouter)(nil)

const defaultProvidersNum = 1

type UnionRouter struct {
	peermgr peermgr.PeerManager
	logger  logrus.FieldLogger

	ctx    context.Context
	cancel context.CancelFunc
}

func New(peermgr peermgr.PeerManager) *UnionRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &UnionRouter{
		peermgr: peermgr,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (u UnionRouter) Start() error {
	u.logger.Infof("Router module started")

	return nil
}

func (u UnionRouter) Stop() error {
	u.cancel()

	u.logger.Infof("Router module stopped")

	return nil
}

func (u UnionRouter) Route(ibtp *pb.IBTP) error {
	providers, err := u.peermgr.FindProviders(ibtp.To, defaultProvidersNum)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if err := u.peermgr.Connect(&provider); err != nil {
			u.logger.WithFields(logrus.Fields{"peerId": ibtp.To,
				"addr": provider.ID.String()}).Error("connect error")
			continue
		}
		data, err := ibtp.Marshal()
		if err != nil {
			return err
		}
		message := peermgr.Message(peerproto.Message_ROUTER_IBTP_SEND, true, data)
		err = u.peermgr.AsyncSend(ibtp.To, message)
		if err != nil {
			return fmt.Errorf("send ibtp error:%v", err)
		}
	}
	return nil
}

func (u UnionRouter) Broadcast(pierIds []string) error {
	for _, peerId := range pierIds {
		if err := u.peermgr.Provider(peerId, true); err != nil {
			return fmt.Errorf("broadcast %s error:%v", peerId, err)
		}
	}
	return nil
}
