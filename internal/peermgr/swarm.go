package peermgr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	network2 "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/network"
	peermgr "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

const (
	protocolID protocol.ID = "/pier/1.0.0" // magic protocol
)

var _ PeerManager = (*Swarm)(nil)

type Swarm struct {
	p2p             network.Network
	logger          logrus.FieldLogger
	peers           map[string]*peer.AddrInfo
	connectedPeers  sync.Map
	msgHandlers     sync.Map
	connectHandlers []ConnectHandler

	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func New(config *repo.Config, privKey crypto.PrivateKey) (*Swarm, error) {
	libp2pPrivKey, err := convertToLibp2pPrivKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("convert private key: %w", err)
	}

	local, remotes, err := loadPeers(config.Mode.Direct.Peers, libp2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("load peers: %w", err)
	}

	p2p, err := network.New(
		network.WithLocalAddr(local),
		network.WithPrivateKey(libp2pPrivKey),
		network.WithProtocolID(protocolID),
	)
	if err != nil {
		return nil, fmt.Errorf("create p2p: %w", err)
	}

	var logger = log.NewWithModule("swarm")

	ctx, cancel := context.WithCancel(context.Background())

	return &Swarm{
		p2p:    p2p,
		logger: logger,
		peers:  remotes,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (swarm *Swarm) Start() error {
	swarm.p2p.SetMessageHandler(swarm.handleMessage)

	if err := swarm.p2p.Start(); err != nil {
		return fmt.Errorf("p2p module start: %w", err)
	}

	for id, addr := range swarm.peers {
		go func(id string, addr *peer.AddrInfo) {
			if err := retry.Retry(func(attempt uint) error {
				if err := swarm.p2p.Connect(addr); err != nil {
					swarm.logger.WithFields(logrus.Fields{
						"node":  id,
						"error": err,
					}).Error("Connect failed")
					return err
				}

				address, err := swarm.getRemoteAddress(addr.ID)
				if err != nil {
					swarm.logger.WithFields(logrus.Fields{
						"node":  id,
						"error": err,
					}).Error("Get remote address failed")
					return err
				}

				swarm.logger.WithFields(logrus.Fields{
					"node":     id,
					"address:": address,
				}).Info("Connect successfully")

				swarm.connectedPeers.Store(address, addr)

				swarm.lock.RLock()
				defer swarm.lock.RUnlock()
				for _, handler := range swarm.connectHandlers {
					go func(connectHandler ConnectHandler, address string) {
						connectHandler(address)
					}(handler, address)
				}

				return nil
			},
				strategy.Wait(1*time.Second),
			); err != nil {
				swarm.logger.Error(err)
			}
		}(id, addr)
	}

	return nil
}

func (swarm *Swarm) Stop() error {
	if err := swarm.p2p.Stop(); err != nil {
		return err
	}

	swarm.cancel()

	return nil
}

func (swarm *Swarm) AsyncSend(id string, msg *peermgr.Message) error {
	addrInfo, err := swarm.getAddrInfo(id)
	if err != nil {
		return fmt.Errorf("async send: %w", err)
	}

	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return swarm.p2p.AsyncSend(addrInfo, network.Message(data))
}

func (swarm *Swarm) SendWithStream(s network2.Stream, msg *peermgr.Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return err
	}

	return swarm.p2p.SendWithStream(s, network.Message(data))
}

func (swarm *Swarm) Send(id string, msg *peermgr.Message) (*peermgr.Message, error) {
	addrInfo, err := swarm.getAddrInfo(id)
	if err != nil {
		return nil, fmt.Errorf("sync send: %w", err)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	ret, err := swarm.p2p.Send(addrInfo, network.Message(data))
	if err != nil {
		return nil, fmt.Errorf("sync send: %w", err)
	}

	m := &peermgr.Message{}
	if err := m.Unmarshal(ret.Data); err != nil {
		return nil, err
	}

	return m, nil
}

func (swarm *Swarm) Peers() map[string]*peer.AddrInfo {
	m := make(map[string]*peer.AddrInfo)
	for id, addr := range swarm.peers {
		m[id] = addr
	}

	return m
}

func (swarm *Swarm) RegisterMsgHandler(messageType peermgr.Message_Type, handler MessageHandler) error {
	if handler == nil {
		return fmt.Errorf("register msg handler: empty handler")
	}

	for msgType := range peermgr.Message_Type_name {
		if msgType == int32(messageType) {
			swarm.msgHandlers.Store(messageType, handler)
			return nil
		}
	}

	return fmt.Errorf("register msg handler: invalid message type")
}

func (swarm *Swarm) RegisterMultiMsgHandler(messageTypes []peermgr.Message_Type, handler MessageHandler) error {
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

func loadPeers(peers []string, privateKey crypto2.PrivKey) (string, map[string]*peer.AddrInfo, error) {
	var local string
	remotes := make(map[string]*peer.AddrInfo)

	id, err := peer.IDFromPrivateKey(privateKey)
	if err != nil {
		return "", nil, err
	}

	for _, p := range peers {
		if strings.HasSuffix(p, id.String()) {
			idx := strings.LastIndex(p, "/p2p/")
			if idx == -1 {
				return "", nil, fmt.Errorf("pid is not existed in bootstrap")
			}

			local = p[:idx]
		} else {
			addr, err := AddrToPeerInfo(p)
			if err != nil {
				return "", nil, fmt.Errorf("wrong network addr: %w", err)
			}
			remotes[addr.ID.String()] = addr
		}
	}

	if local == "" {
		return "", nil, fmt.Errorf("get local addr: no local addr is configured")
	}

	return local, remotes, nil
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

func (swarm *Swarm) handleMessage(s network2.Stream, data []byte) {
	m := &peermgr.Message{}
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

// TODO
func (swarm *Swarm) getRemoteAddress(id peer.ID) (string, error) {
	//pubKey, err := swarm.p2p.GetRemotePubKey(id)
	//if err != nil {
	//	return "", err
	//}
	//
	//raw, err := pubKey.Raw()
	//if err != nil {
	//	return "", err
	//}
	//
	//key, err := x509.ParsePKIXPublicKey(raw)
	//if err != nil {
	//	return "", err
	//}
	//
	//publicKey := ecdsa.NewPublicKey(key.(*ecdsa2.PublicKey))
	//addr, err := publicKey.Address()
	//if err != nil {
	//	return "", err
	//}
	//
	//return addr.String(), nil
	return "", nil
}

func (swarm *Swarm) RegisterConnectHandler(handler ConnectHandler) error {
	swarm.lock.Lock()
	defer swarm.lock.Unlock()

	swarm.connectHandlers = append(swarm.connectHandlers, handler)

	return nil
}
