package peermgr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/ipfs/go-cid"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	basicMgr "github.com/meshplus/bitxhub-core/peer-mgr"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/repo"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

const (
	protocolID          = "/pier/1.0.0" // magic protocol
	defaultProvidersNum = 1
)

var _ PeerManager = (*Swarm)(nil)

type Swarm struct {
	p2p             network.Network
	logger          logrus.FieldLogger
	peers           map[string]*peer.AddrInfo // key is pid, value is AddrInfo
	connectedPeers  sync.Map                  // key is appChain address, value is AddrInfo(including pid and addr)
	msgHandlers     sync.Map
	providers       uint64
	connectHandlers []ConnectHandler
	privKey         crypto.PrivateKey

	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (swarm *Swarm) Peers() map[string]*peer.AddrInfo {
	m := make(map[string]*peer.AddrInfo)
	for _, addr := range swarm.peers {
		pierID, err := swarm.getRemotePierID(addr.ID)
		if err != nil {
			return nil
		}
		m[pierID] = addr
	}

	return m
}

func New(config *repo.Config, nodePrivKey crypto.PrivateKey, privKey crypto.PrivateKey, providers uint64, logger logrus.FieldLogger) (*Swarm, error) {
	libp2pPrivKey, err := convertToLibp2pPrivKey(nodePrivKey)
	if err != nil {
		return nil, fmt.Errorf("convert node private key: %w", err)
	}

	networkConfiig, err := repo.LoadNetworkConfig(config.RepoRoot, libp2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("load peers: %w", err)
	}

	remotes := make(map[string]*peer.AddrInfo)
	id, err := peer.IDFromPrivateKey(libp2pPrivKey)
	p2pPeers, _ := repo.GetNetworkPeers(networkConfiig)
	for pid, addrInfo := range p2pPeers {
		if strings.Compare(pid, id.String()) == 0 {
			continue
		}
		remotes[pid] = addrInfo
	}

	var protocolIDs = []string{protocolID}

	p2p, err := network.New(
		network.WithLocalAddr(networkConfiig.LocalAddr),
		network.WithPrivateKey(libp2pPrivKey),
		network.WithProtocolIDs(protocolIDs),
		network.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("create p2p: %w", err)
	}

	if providers == 0 {
		providers = defaultProvidersNum
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Swarm{
		providers: providers,
		p2p:       p2p,
		logger:    logger,
		peers:     remotes,
		privKey:   privKey,
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

func (swarm *Swarm) Start() error {
	swarm.p2p.SetMessageHandler(swarm.handleMessage)

	if err := swarm.p2p.Start(); err != nil {
		return fmt.Errorf("p2p module start: %w", err)
	}

	//need to connect one other pier at least
	wg := &sync.WaitGroup{}
	wg.Add(1)

	for id, addr := range swarm.peers {
		go func(id string, addr *peer.AddrInfo) {
			if err := retry.Retry(func(attempt uint) error {
				if err := swarm.p2p.Connect(*addr); err != nil {
					if attempt != 0 && attempt%5 == 0 {
						swarm.logger.WithFields(logrus.Fields{
							"node":  id,
							"error": err,
						}).Error("Connect failed")
					}
					return err
				}

				pierId, err := swarm.getRemotePierID(addr.ID)
				if err != nil {
					swarm.logger.WithFields(logrus.Fields{
						"node":  id,
						"error": err,
					}).Error("Get remote address failed")
					return err
				}

				swarm.logger.WithFields(logrus.Fields{
					"node":     id,
					"address:": pierId,
				}).Info("Connect successfully")

				swarm.connectedPeers.Store(pierId, addr)

				swarm.lock.RLock()
				defer swarm.lock.RUnlock()
				for _, handler := range swarm.connectHandlers {
					go func(connectHandler ConnectHandler, address string) {
						connectHandler(address)
					}(handler, pierId)
				}
				wg.Done()
				return nil
			},
				strategy.Wait(1*time.Second),
			); err != nil {
				swarm.logger.Error(err)
			}
		}(id, addr)
	}

	wg.Wait()

	return nil
}

func (swarm *Swarm) Stop() error {
	if err := swarm.p2p.Stop(); err != nil {
		return err
	}

	swarm.cancel()

	return nil
}

func (swarm *Swarm) AsyncSend(id basicMgr.KeyType, msg *pb.Message) error {
	addrInfo, err := swarm.getAddrInfo(id.(string))
	if err != nil {
		return err
	}
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return swarm.p2p.AsyncSend(addrInfo.ID.String(), data)
}

func (swarm *Swarm) SendWithStream(s network.Stream, msg *pb.Message) (*pb.Message, error) {
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	recvData, err := s.Send(data)
	if err != nil {
		return nil, err
	}
	recvMsg := &pb.Message{}
	if err := recvMsg.Unmarshal(recvData); err != nil {
		return nil, err
	}
	return recvMsg, nil
}

func (swarm *Swarm) Connect(addrInfo *peer.AddrInfo) (string, error) {
	err := swarm.p2p.Connect(*addrInfo)
	if err != nil {
		return "", err
	}
	pierId, err := swarm.getRemotePierID(addrInfo.ID)
	if err != nil {
		return "", err
	}
	swarm.logger.WithFields(logrus.Fields{
		"pierId":   pierId,
		"addrInfo": addrInfo,
	}).Info("Connect peer")
	swarm.connectedPeers.Store(pierId, addrInfo)
	return pierId, nil
}

func (swarm *Swarm) AsyncSendWithStream(s network.Stream, msg *pb.Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return err
	}

	return s.AsyncSend(data)
}

func (swarm *Swarm) Send(id basicMgr.KeyType, msg *pb.Message) (*pb.Message, error) {
	addrInfo, err := swarm.getAddrInfo(id.(string))
	if err != nil {
		return nil, err
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	ret, err := swarm.p2p.Send(addrInfo.ID.String(), data)
	if err != nil {
		return nil, fmt.Errorf("sync send: %w", err)
	}

	m := &pb.Message{}
	if err := m.Unmarshal(ret); err != nil {
		return nil, err
	}

	return m, nil
}

// ConnectedPeerIDs gets connected PeerIDs
// TODO
func (swarm *Swarm) ConnectedPeerIDs() []string {
	peerIDs := []string{}
	swarm.connectedPeers.Range(func(key, value interface{}) bool {
		peerIDs = append(peerIDs, value.(string))
		return true
	})
	return peerIDs
}

func (swarm *Swarm) CountConnectedPeers() uint64 {
	var counter uint64
	swarm.connectedPeers.Range(func(k, v interface{}) bool {
		counter++
		return true
	})
	return counter
}

func (swarm *Swarm) RegisterMsgHandler(messageType pb.Message_Type, handler MessageHandler) error {
	if handler == nil {
		return fmt.Errorf("register msg handler: empty handler")
	}

	for msgType := range pb.Message_Type_name {
		if msgType == int32(messageType) {
			swarm.msgHandlers.Store(messageType, handler)
			return nil
		}
	}

	return fmt.Errorf("register msg handler: invalid message type")
}

func (swarm *Swarm) RegisterMultiMsgHandler(messageTypes []pb.Message_Type, handler MessageHandler) error {
	for _, typ := range messageTypes {
		if err := swarm.RegisterMsgHandler(typ, handler); err != nil {
			return err
		}
	}

	return nil
}

func (swarm *Swarm) getAddrInfo(id string) (*peer.AddrInfo, error) {
	addr, ok := swarm.connectedPeers.Load(id)
	if !ok {
		return nil, fmt.Errorf("wrong id: %s", id)
	}

	return addr.(*peer.AddrInfo), nil
}

func convertToLibp2pPrivKey(privateKey crypto.PrivateKey) (crypto2.PrivKey, error) {
	ecdsaPrivKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("convert to libp2p private key: not ecdsa private key")
	}

	libp2pPrivKey, _, err := crypto2.ECDSAKeyPairFromKey(ecdsaPrivKey.K)
	if err != nil {
		return nil, err
	}

	return libp2pPrivKey, nil
}

// AddrToPeerInfo transfer addr to PeerInfo
// addr example: "/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64"
func AddrToPeerInfo(multiAddr string) (*peer.AddrInfo, error) {
	maddr, err := ma.NewMultiaddr(multiAddr)
	if err != nil {
		return nil, err
	}

	return peer.AddrInfoFromP2pAddr(maddr)
}

func (swarm *Swarm) handleMessage(s network.Stream, data []byte) {
	m := &pb.Message{}
	if err := m.Unmarshal(data); err != nil {
		swarm.logger.Error(err)
		return
	}

	handler, ok := swarm.msgHandlers.Load(m.Type)
	if !ok {
		swarm.logger.WithFields(logrus.Fields{
			"error": fmt.Errorf("can't handle msg[type: %v]", m.Type),
			"type":  m.Type.String(),
		}).Error("Handle message")
		return
	}

	msgHandler, ok := handler.(MessageHandler)
	if !ok {
		swarm.logger.WithFields(logrus.Fields{
			"error": fmt.Errorf("invalid handler for msg [type: %v]", m.Type),
			"type":  m.Type.String(),
		}).Error("Handle message")
		return
	}

	msgHandler(s, m)
}

func (swarm *Swarm) getRemotePierID(id peer.ID) (string, error) {
	msg := Message(pb.Message_ADDRESS_GET, true, nil)
	reqData, err := msg.Marshal()
	if err != nil {
		return "", err
	}
	retData, err := swarm.p2p.Send(id.String(), reqData)
	if err != nil {
		return "", fmt.Errorf("sync send: %w", err)
	}
	ret := &pb.Message{}
	if err := ret.Unmarshal(retData); err != nil {
		return "", err
	}

	return string(DataToPayload(ret).Data), nil
}

func (swarm *Swarm) RegisterConnectHandler(handler ConnectHandler) error {
	swarm.lock.Lock()
	defer swarm.lock.Unlock()

	swarm.connectHandlers = append(swarm.connectHandlers, handler)

	return nil
}

func (swarm *Swarm) FindProviders(id string) (string, error) {
	format := cid.V0Builder{}
	toCid, err := format.Sum([]byte(id))
	if err != nil {
		return "", err
	}
	providers, err := swarm.p2p.FindProvidersAsync(toCid.String(), int(swarm.providers))
	if err != nil {
		swarm.logger.WithFields(logrus.Fields{
			"id": id,
		}).Error("Not find providers")
		return "", err
	}

	for provider := range providers {
		swarm.logger.WithFields(logrus.Fields{
			"id":          id,
			"cid":         toCid.String(),
			"provider_id": provider.ID.String(),
		}).Info("Find provider")

		pierId, err := swarm.Connect(&provider)
		if err != nil {
			swarm.logger.WithFields(logrus.Fields{"peerId": pierId,
				"cid": provider.ID.String()}).Error("connect error: ", err)
			continue
		}
		return pierId, nil
	}

	swarm.logger.WithFields(logrus.Fields{
		"id":  id,
		"cid": toCid.String(),
	}).Warning("No providers found") // TODO add error
	return "", nil
}

func (swarm *Swarm) Provider(key string, passed bool) error {
	return swarm.p2p.Provider(key, passed)
}
