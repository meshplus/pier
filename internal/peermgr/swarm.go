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
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/bitxid"
	network "github.com/meshplus/go-lightp2p"
	network2 "github.com/meshplus/go-lightp2p/hybrid"
	network_pb "github.com/meshplus/go-lightp2p/pb"
	"github.com/meshplus/pier/internal/repo"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

const (
	//protocolID          protocol.ID = "/pier/1.0.0" // magic protocol
	protocolID          protocol.ID = "/pangolin/1.0.0" // magic protocol
	defaultProvidersNum             = 1
	SubscribeResponse               = "Successfully subscribe"
	PangolinID                      = "pangolin"
)

var _ PeerManager = (*Swarm)(nil)

type Swarm struct {
	p2p       network2.HybridNetwork
	localAddr string
	// 自己连接的pangolin地址
	pangolinAddr string
	logger       logrus.FieldLogger
	peers        map[string]*peer.AddrInfo
	// 需要connect的pangolin地址（bxh端pangolin的地址）
	pangolinConnect string
	// 通过pangolin连接的bxh地址
	pangolinPeers     map[string]string
	connectedPeers    sync.Map
	connectedBxhPeers sync.Map
	msgHandlers       sync.Map
	providers         uint64
	connectHandlers   []ConnectHandler
	privKey           crypto.PrivateKey

	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (swarm *Swarm) GetPangolinAddr() string {
	return swarm.pangolinAddr
}

func (swarm *Swarm) GetLocalAddr() string {
	return swarm.localAddr
}

func (swarm *Swarm) CheckMasterPier(address string) (*pb.Response, error) {
	request := &pb.Address{
		Address: address,
	}
	rb, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	msg := Message(pb.Message_PIER_CHECK_MASTER_PIER, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("CheckMasterPier marshal err: %w", err)
	}
	res, err := swarm.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("CheckMasterPier from bitxhub: %w", err)
	}
	response := &pb.Response{}
	err = response.Unmarshal(res)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (swarm *Swarm) SetMasterPier(address string, index string, timeout int64) (*pb.Response, error) {
	request := &pb.PierInfo{
		Address: address,
		Index:   index,
		Timeout: timeout,
	}

	rb, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	msg := Message(pb.Message_PIER_SET_MASTER_PIER, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("SetMasterPier marshal err: %w", err)
	}
	res, err := swarm.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("SetMasterPier from bitxhub: %w", err)
	}
	response := &pb.Response{}
	err = response.Unmarshal(res)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (swarm *Swarm) HeartBeat(address string, index string) (*pb.Response, error) {
	request := &pb.PierInfo{
		Address: address,
		Index:   index,
	}
	rb, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	msg := Message(pb.Message_PIER_HEART_BEAT, true, rb)
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("HeartBeat marshal err: %w", err)
	}
	res, err := swarm.SendByMultiAddr(data)
	if err != nil {
		return nil, fmt.Errorf("HeartBeat from bitxhub: %w", err)
	}
	response := &pb.Response{}
	err = response.Unmarshal(res)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (swarm *Swarm) AsyncSendByMultiAddr(data []byte) error {
	var success bool
	swarm.connectedBxhPeers.Range(func(key, value interface{}) bool {
		multiAddr := value.(string)
		err := swarm.p2p.AsyncSendByMultiAddr(multiAddr, data)
		if err != nil {
			swarm.logger.WithFields(logrus.Fields{
				"node":  multiAddr,
				"error": err,
			}).Error("SendByMultiAddr failed")
			return true
		}
		success = true
		// if successfully send msg, stop the range
		return false
	})
	if !success {
		return fmt.Errorf("AsyncSendByMultiAddr err: all multiAddr connect failed")
	}
	return nil
}

func (swarm *Swarm) SendByMultiAddr(data []byte) ([]byte, error) {
	var (
		resp    []byte
		err     error
		success bool
	)
	if err := retry.Retry(func(attempt uint) error {
		swarm.connectedBxhPeers.Range(func(key, value interface{}) bool {
			multiAddr := value.(string)
			bxId := key.(string)
			//hexData := hexutil.Encode(data)
			//resp, err = swarm.p2p.SendByMultiAddr(multiAddr, []byte(hexData))
			resp, err = swarm.p2p.SendByMultiAddr(multiAddr, data)
			if err != nil {
				swarm.logger.WithFields(logrus.Fields{
					"node":  bxId,
					"error": err,
				}).Error("SendByMultiAddr failed")
				return true
			}
			// if successfully send msg, stop the range
			success = true
			return false
		})
		if !success {
			return fmt.Errorf("AsyncSendByMultiAddr err: all multiAddr connect failed")
		}

		return nil
	}, strategy.Wait(500*time.Millisecond)); err != nil {
		swarm.logger.Panic(err)
	}
	return resp, nil
}

func New(config *repo.Config, nodePrivKey crypto.PrivateKey, privKey crypto.PrivateKey, providers uint64, logger logrus.FieldLogger) (*Swarm, error) {
	libp2pPrivKey, err := convertToLibp2pPrivKey(nodePrivKey)
	if err != nil {
		return nil, fmt.Errorf("convert private key: %w", err)
	}
	var local string
	var remotes map[string]*peer.AddrInfo
	var pangolinPeers map[string]string
	var pangolinConnected string
	switch config.Mode.Type {
	case repo.UnionMode:
		local, remotes, err = loadPeers(config.Mode.Union.Connectors, libp2pPrivKey)
		if err != nil {
			return nil, fmt.Errorf("load peers: %w", err)
		}
	case repo.DirectMode:
		local, remotes, err = loadPeers(config.Mode.Direct.Peers, libp2pPrivKey)
		if err != nil {
			return nil, fmt.Errorf("load peers: %w", err)
		}
	case repo.RelayMode:
		local, pangolinConnected, pangolinPeers, err = loadPangolinPeers(config.Mode.Relay.PangolinAddrs,
			config.Mode.Relay.BxhAddrs, config.Mode.Relay.PierAddr)
	default:
		return nil, fmt.Errorf("unsupport mode type")
	}

	p2p, err := network2.NewHybridNet(
		network2.WithLocalAddr(local),
		network2.WithPrivateKey(libp2pPrivKey),
		network2.WithProtocolID(protocolID),
		network2.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("create p2p: %w", err)
	}

	if providers == 0 {
		providers = defaultProvidersNum
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Swarm{
		providers:       providers,
		p2p:             p2p,
		localAddr:       config.Mode.Relay.PierAddr,
		pangolinAddr:    config.Mode.Relay.PangolinAddrs[0],
		logger:          logger,
		peers:           remotes,
		pangolinConnect: pangolinConnected,
		pangolinPeers:   pangolinPeers,
		privKey:         privKey,
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

func (swarm *Swarm) Start() error {
	swarm.p2p.SetMessageHandler(swarm.handleMessage)

	if err := swarm.RegisterMsgHandler(pb.Message_ADDRESS_GET, swarm.handleGetAddressMessage); err != nil {
		return fmt.Errorf("register get address msg handler: %w", err)
	}

	if err := swarm.p2p.Start(); err != nil {
		return fmt.Errorf("p2p module start: %w", err)
	}

	//need to connect one other pier at least
	wg := &sync.WaitGroup{}
	wg.Add(len(swarm.peers) + len(swarm.pangolinPeers))

	for id, addrInfo := range swarm.peers {
		go func(id string, addrInfo *peer.AddrInfo) {
			defer wg.Done()
			if err := retry.Retry(func(attempt uint) error {
				if err := swarm.p2p.Connect(addrInfo); err != nil {
					if attempt != 0 && attempt%5 == 0 {
						swarm.logger.WithFields(logrus.Fields{
							"node":  id,
							"error": err,
						}).Error("Connect failed")
					}
					return err
				}

				pierID, err := swarm.getRemoteAddress(addrInfo)
				if err != nil {
					swarm.logger.WithFields(logrus.Fields{
						"node":  id,
						"error": err,
					}).Error("Get remote address failed")
					return err
				}

				swarm.logger.WithFields(logrus.Fields{
					"node":     id,
					"address:": pierID,
				}).Info("Connect successfully")

				swarm.connectedPeers.Store(pierID, addrInfo)

				swarm.lock.RLock()
				defer swarm.lock.RUnlock()
				for _, handler := range swarm.connectHandlers {
					go func(connectHandler ConnectHandler, address string) {
						connectHandler(address)
					}(handler, pierID)
				}
				return nil
			},
				strategy.Wait(1*time.Second),
			); err != nil {
				swarm.logger.Error(err)
			}
		}(id, addrInfo)
	}

	// connect bxh pangolin
	if err := retry.Retry(func(attempt uint) error {
		if err := swarm.p2p.ConnectByMultiAddr(swarm.pangolinConnect); err != nil {
			if attempt != 0 && attempt%5 == 0 {
				swarm.logger.WithFields(logrus.Fields{
					"node":  PangolinID,
					"error": err,
				}).Error("Pangolin Connect failed")
			}
			return err
		}

		return nil
	},
		strategy.Limit(5),
		strategy.Wait(1*time.Second),
	); err != nil {
		swarm.logger.Error(err)
	}
	swarm.logger.Infof("Connect pangolin successfully")

	// connect bxh nodes
	for id, addr := range swarm.pangolinPeers {
		go func(id string, addr string) {
			defer wg.Done()
			if err := retry.Retry(func(attempt uint) error {
				if err := swarm.p2p.ConnectByMultiAddr(addr); err != nil {
					if attempt != 0 && attempt%5 == 0 {
						swarm.logger.WithFields(logrus.Fields{
							"node":  id,
							"error": err,
						}).Error("Connect bxh node failed")
					}
					return err
				}
				swarm.connectedBxhPeers.Store(id, addr)
				swarm.logger.WithFields(logrus.Fields{
					"node":     id,
					"address:": addr,
				}).Info("Connect bxh node successfully")

				swarm.lock.RLock()
				defer swarm.lock.RUnlock()
				for _, handler := range swarm.connectHandlers {
					go func(connectHandler ConnectHandler, address string) {
						connectHandler(address)
					}(handler, addr)
				}
				return nil
			},
				strategy.Wait(1*time.Second),
			); err != nil {
				swarm.logger.Error(err)
			}
		}(id, addr)
	}

	wg.Wait()

	swarm.logger.Infof("successful start swarm")
	return nil
}

func (swarm *Swarm) Stop() error {
	if err := swarm.p2p.Stop(); err != nil {
		return err
	}

	swarm.cancel()

	return nil
}

func (swarm *Swarm) AsyncSend(id string, msg *pb.Message) error {
	addrInfo, err := swarm.getAddrInfo(id)
	if err != nil {
		return err
	}
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	msg1 := &network_pb.Message{
		Data: data,
		Msg:  network_pb.Message_DATA_ASYNC,
	}
	return swarm.p2p.AsyncSend(addrInfo, msg1)
}

// SendWithStream
// todo(lrx) not implement correctly
func (swarm *Swarm) SendWithStream(s network.Stream, msg *pb.Message) (*pb.Message, error) {
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}
	//msg1 := &network_pb.Message{
	//	Data: data,
	//	Msg:  network_pb.Message_DATA,
	//}
	recvData, err := swarm.p2p.SendWithAliveStream(s, data)
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
	err := swarm.p2p.Connect(addrInfo)
	if err != nil {
		return "", err
	}
	pierId, err := swarm.getRemoteAddress(addrInfo)
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

func (swarm *Swarm) Send(id string, msg *pb.Message) (*pb.Message, error) {
	addrInfo, err := swarm.getAddrInfo(id)
	if err != nil {
		return nil, err
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	msg1 := &network_pb.Message{
		Data: data,
		Msg:  network_pb.Message_DATA,
	}

	ret, err := swarm.p2p.Send(addrInfo, msg1)
	if err != nil {
		return nil, fmt.Errorf("sync send: %w", err)
	}

	m := &pb.Message{}
	if err := m.Unmarshal(ret.Data); err != nil {
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

func (swarm *Swarm) Peers() map[string]*peer.AddrInfo {
	m := make(map[string]*peer.AddrInfo)
	for id, addr := range swarm.peers {
		m[id] = addr
	}

	return m
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

// todo(lrx) refactor addrInfo
func (swarm *Swarm) getAddrInfo(id string) (*peer.AddrInfo, error) {
	if bitxid.DID(id).IsValidFormat() {
		id = strings.TrimPrefix(bitxid.DID(id).GetSubMethod(), "appchain")
	}
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

func loadPangolinPeers(pangolinAddrs, bxhAddrs []string, pierAddr string) (string, string, map[string]string, error) {
	remotes := make(map[string]string)

	if len(pangolinAddrs) != 2 {
		return "", "", nil, fmt.Errorf("pangolin addrs err: the length should be 2, "+
			"but actually is %d", len(pangolinAddrs))
	}
	local, err := AddrToPeerInfo(pierAddr)
	if err != nil {
		return "", "", nil, fmt.Errorf("wrong pier network addr: %w", err)
	}

	remote := fmt.Sprintf("/peer" + pangolinAddrs[0] + "/netgap" + pangolinAddrs[0] + "/peer" + pangolinAddrs[1])
	if _, err = network2.StrToMultiAddr(remote); err != nil {
		return "", "", nil, fmt.Errorf("wrong pangolin multi network addr: %w", err)
	}

	for _, b := range bxhAddrs {
		addr := fmt.Sprintf("/peer" + pangolinAddrs[0] + "/netgap" + pangolinAddrs[0] + "/peer" + pangolinAddrs[1] + "/peer" + b)
		bxhAddr, err := AddrToPeerInfo(b)
		if err != nil {
			return "", "", nil, fmt.Errorf("wrong bitxhub network addr: %w", err)
		}
		if _, err = network2.StrToMultiAddr(addr); err != nil {
			return "", "", nil, fmt.Errorf("wrong pangolin multi network addr: %w", err)
		}
		remotes[bxhAddr.ID.String()] = addr
	}
	return local.Addrs[0].String(), remote, remotes, nil
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

func (swarm *Swarm) getRemoteAddress(addr *peer.AddrInfo) (string, error) {
	msg := Message(pb.Message_ADDRESS_GET, true, nil)
	reqData, err := msg.Marshal()
	if err != nil {
		return "", err
	}
	msg1 := &network_pb.Message{
		Data: reqData,
		Msg:  network_pb.Message_DATA,
	}
	retData, err := swarm.p2p.Send(addr, msg1)
	if err != nil {
		return "", fmt.Errorf("sync send: %w", err)
	}
	ret := &pb.Message{}
	if err := ret.Unmarshal(retData.Data); err != nil {
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

func (swarm *Swarm) handleGetAddressMessage(stream network.Stream, message *pb.Message) {
	addr, err := swarm.privKey.PublicKey().Address()
	if err != nil {
		swarm.logger.Error(err)
		return
	}

	retMsg := Message(pb.Message_ACK, true, []byte(addr.String()))

	err = swarm.AsyncSendWithStream(stream, retMsg)
	if err != nil {
		swarm.logger.Error(err)
	}
}
