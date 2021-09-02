package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/syncer"
	"github.com/meshplus/pier/internal/utils"
	"github.com/sirupsen/logrus"
)

var _ Router = (*UnionRouter)(nil)

type UnionRouter struct {
	peermgr          peermgr.PeerManager
	syncer           syncer.Syncer
	logger           logrus.FieldLogger
	store            storage.Storage
	appchains        map[string]*appchainmgr.Appchain
	pbTable          sync.Map
	connectedPierIDs []string

	ctx    context.Context
	cancel context.CancelFunc
}

func New(peermgr peermgr.PeerManager, store storage.Storage, logger logrus.FieldLogger, connectedPierIDs []string) *UnionRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &UnionRouter{
		peermgr:          peermgr,
		store:            store,
		appchains:        make(map[string]*appchainmgr.Appchain),
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
		connectedPierIDs: connectedPierIDs,
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
	data, err := ibtp.Marshal()
	if err != nil {
		return err
	}

	message := peermgr.Message(peerproto.Message_ROUTER_IBTP_SEND, true, data)

	_, target, err := utils.GetSrcDstBitXHubID(ibtp.ID(), ibtp.Category() == pb.IBTP_REQUEST)
	if err != nil {
		return err
	}

	handle := func() error {
		pierId, err := u.peermgr.FindProviders(target)
		if err != nil {
			return err
		}
		res, err := u.peermgr.Send(pierId, message)
		if err != nil || res.Type != peerproto.Message_ACK || !res.Payload.Ok {
			u.logger.Errorf("send ibtp error:%v", err)
			return err
		}
		u.pbTable.Store(target, pierId)
		u.store.Put(RouteIBTPKey(ibtp.ID()), []byte(""))
		u.logger.WithField("ibtp", ibtp.ID()).Infof("send ibtp successfully from %s to %s", ibtp.From, ibtp.To)
		return nil
	}

	//find target union pier by local cache
	if unionPierId, ok := u.pbTable.Load(target); ok {
		res, err := u.peermgr.Send(unionPierId.(string), message)
		if err == nil && res.Type == peerproto.Message_ACK && res.Payload.Ok {
			u.store.Put(RouteIBTPKey(ibtp.ID()), []byte(""))
			u.logger.WithField("ibtp", ibtp.ID()).Infof("send ibtp successfully from %s to %s", ibtp.From, ibtp.To)
			return nil
		}
	}

	if err := handle(); err != nil {
		u.pbTable.Delete(target)
		u.logger.Errorf("send ibtp %s with category %s error: %v", ibtp.ID(), ibtp.Category().String(), err)
		return err
	}
	u.store.Put(RouteIBTPKey(ibtp.ID()), []byte(""))

	return nil
}

//Broadcast broadcasts current BitXHub Chain ID to the union network
func (u *UnionRouter) Broadcast(bxhID string) error {
	// Construct v0 cid
	format := cid.V0Builder{}
	idCid, err := format.Sum([]byte(bxhID))
	if err != nil {
		return err
	}

	if err := u.peermgr.Provider(idCid.String(), true); err != nil {
		return fmt.Errorf("broadcast %s error:%w", bxhID, err)
	}
	u.logger.WithFields(logrus.Fields{
		"bxhID": bxhID,
		"cid":   idCid.String(),
	}).Info("provide cid to network")

	return nil
}

func (u *UnionRouter) QueryInterchain(bxhID, serviceID string) (*pb.Interchain, error) {
	message := peermgr.Message(peerproto.Message_ROUTER_INTERCHAIN_GET, true, []byte(serviceID))

	interchain := &pb.Interchain{}

	//find target union pier by local cache
	if unionPierId, ok := u.pbTable.Load(bxhID); ok {
		res, err := u.peermgr.Send(unionPierId.(string), message)
		if err == nil && res.Type == peerproto.Message_ACK && res.Payload.Ok {
			if err := interchain.Unmarshal(res.Payload.Data); err == nil {
				u.logger.WithFields(logrus.Fields{
					"bxhID":     bxhID,
					"serviceID": serviceID,
				}).Info("Get interchain successfully")
				return interchain, nil
			}
		} else {
			u.pbTable.Delete(bxhID)
		}
	}

	handle := func() error {
		pierId, err := u.peermgr.FindProviders(bxhID)
		if err != nil {
			return err
		}
		res, err := u.peermgr.Send(pierId, message)
		if err != nil || res.Type != peerproto.Message_ACK || !res.Payload.Ok {
			u.logger.Errorf("get interchain error:%v", err)
			return err
		}
		if err := interchain.Unmarshal(res.Payload.Data); err != nil {
			return err
		}
		u.pbTable.Store(bxhID, pierId)
		u.logger.WithFields(logrus.Fields{
			"bxhID":     bxhID,
			"serviceID": serviceID,
		}).Info("Get interchain successfully")
		return nil
	}

	if err := handle(); err != nil {
		return nil, err
	}

	return interchain, nil
}

func (u *UnionRouter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	target, _, err := utils.GetSrcDstBitXHubID(id, isReq)
	if err != nil {
		return nil, err
	}

	message := peermgr.Message(peerproto.Message_ROUTER_IBTP_GET, true, []byte(id))
	if !isReq {
		message = peermgr.Message(peerproto.Message_ROUTER_IBTP_RECEIPT_GET, true, []byte(id))
	}

	ibtp := &pb.IBTP{}

	//find target union pier by local cache
	if unionPierId, ok := u.pbTable.Load(target); ok {
		res, err := u.peermgr.Send(unionPierId.(string), message)
		if err == nil && res.Type == peerproto.Message_ACK && res.Payload.Ok {
			if err := ibtp.Unmarshal(res.Payload.Data); err == nil {
				u.logger.WithFields(logrus.Fields{
					"ibtp":  ibtp.ID(),
					"isReq": isReq,
				}).Info("Get ibtp successfully")
				return ibtp, nil
			}
		} else {
			u.pbTable.Delete(target)
		}
	}

	handle := func() error {
		u.logger.Infof("target is %s for ibtp %s, isReq: %v", target, id, isReq)
		pierId, err := u.peermgr.FindProviders(target)
		if err != nil {
			return err
		}
		res, err := u.peermgr.Send(pierId, message)
		if err != nil || res.Type != peerproto.Message_ACK || !res.Payload.Ok {
			u.logger.Errorf("get ibtp error:%v", err)
			return err
		}
		if err := ibtp.Unmarshal(res.Payload.Data); err != nil {
			return err
		}
		u.pbTable.Store(target, pierId)
		u.logger.WithFields(logrus.Fields{
			"ibtp":  ibtp.ID(),
			"isReq": isReq,
		}).Info("Get ibtp successfully")
		return nil
	}

	if err := handle(); err != nil {
		return nil, err
	}

	return ibtp, nil
}

func RouteIBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("route-ibtp-%s", id))
}
