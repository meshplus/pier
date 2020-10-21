package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/meshplus/bitxhub-kit/storage"

	"github.com/ipfs/go-cid"

	"github.com/meshplus/pier/internal/syncer"

	rpcx "github.com/meshplus/go-bitxhub-client"

	peerproto "github.com/meshplus/pier/internal/peermgr/proto"

	"github.com/meshplus/bitxhub-kit/log"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("union_router")
var _ Router = (*UnionRouter)(nil)

type UnionRouter struct {
	peermgr   peermgr.PeerManager
	syncer    syncer.Syncer
	logger    logrus.FieldLogger
	store     storage.Storage
	appchains map[string]*rpcx.Appchain
	pbTable   sync.Map

	ctx    context.Context
	cancel context.CancelFunc
}

func New(peermgr peermgr.PeerManager, store storage.Storage) *UnionRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &UnionRouter{
		peermgr:   peermgr,
		store:     store,
		appchains: make(map[string]*rpcx.Appchain),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (u *UnionRouter) Start() error {
	u.logger.Infof("Router module started")

	return nil
}

func (u *UnionRouter) Stop() error {
	u.cancel()

	u.logger.Infof("Router module stopped")

	return nil
}

//Route sends ibtp to the union pier in target relay chain
func (u *UnionRouter) Route(ibtp *pb.IBTP) error {
	if ok := u.store.Has(RouteIBTPKey(ibtp.ID())); ok {
		u.logger.WithField("ibtp", ibtp.ID()).Info("IBTP has routed by this pier")
		return nil
	}

	data, err := ibtp.Marshal()
	if err != nil {
		return err
	}

	message := peermgr.Message(peerproto.Message_ROUTER_IBTP_SEND, true, data)

	handle := func() error {
		pierId, err := u.peermgr.FindProviders(ibtp.To)
		if err != nil {
			return err
		}
		res, err := u.peermgr.Send(pierId, message)
		if err != nil || res.Type != peerproto.Message_ACK || !res.Payload.Ok {
			u.logger.Errorf("send ibtp error:%v", err)
		}
		u.pbTable.Store(ibtp.To, pierId)
		u.store.Put([]byte(ibtp.ID()), []byte(""))
		return nil
	}

	//find target union pier by local cache
	if unionPierId, ok := u.pbTable.Load(ibtp.To); ok {
		res, err := u.peermgr.Send(unionPierId.(string), message)
		if err == nil && res.Type == peerproto.Message_ACK && res.Payload.Ok {
			u.store.Put(RouteIBTPKey(ibtp.ID()), []byte(""))
			return nil
		}
	}

	if err := handle(); err != nil {
		u.pbTable.Delete(ibtp.To)
		u.logger.Errorf("send ibtp error:%v", err)
		return err
	}
	u.store.Put(RouteIBTPKey(ibtp.ID()), []byte(""))

	return nil
}

//Broadcast broadcasts the registered appchain ids to the union network
func (u *UnionRouter) Broadcast(appchainIds []string) error {
	for _, id := range appchainIds {
		// Construct v0 cid
		format := cid.V0Builder{}
		idCid, err := format.Sum([]byte(id))
		if err != nil {
			return err
		}

		if err := u.peermgr.Provider(idCid.String(), true); err != nil {
			return fmt.Errorf("broadcast %s error:%w", id, err)
		}
		u.logger.WithFields(logrus.Fields{
			"id":  id,
			"cid": idCid.String(),
		}).Info("provider cid")
	}
	return nil
}

//AddAppchains adds appchains to route map and broadcast them to union network
func (u *UnionRouter) AddAppchains(appchains []*rpcx.Appchain) error {
	if len(appchains) == 0 {
		return nil
	}

	ids := make([]string, 0)
	for _, appchain := range appchains {
		if _, ok := u.appchains[appchain.ID]; ok {
			continue
		}
		ids = append(ids, appchain.ID)
		u.appchains[appchain.ID] = appchain
	}
	if len(ids) == 0 {
		return nil
	}
	return u.Broadcast(ids)
}

//ExistAppchain returns if appchain id exit in route map
func (u *UnionRouter) ExistAppchain(id string) bool {
	_, ok := u.appchains[id]
	return ok
}

func RouteIBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("route-ibtp-%s", id))
}
