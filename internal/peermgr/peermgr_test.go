package peermgr

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	libp2pcry "github.com/libp2p/go-libp2p-core/crypto"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/repo"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestSwarm_Peers(t *testing.T) {
	logger := log.NewWithModule("swarm")
	repoRoot, err := ioutil.TempDir("", "node")
	require.Nil(t, err)
	defer os.RemoveAll(repoRoot)

	nodeKeys, privKeys, repoConfig, _ := genKeysAndConfig(t, repoRoot, 2)

	swarm1, err := New(repoConfig.Config, nodeKeys[0], privKeys[0], 0, logger)

	swarm1.Peers()
}

func TestNew(t *testing.T) {
	logger := log.NewWithModule("swarm")
	repoRoot, err := ioutil.TempDir("", "node")
	require.Nil(t, err)
	defer os.RemoveAll(repoRoot)

	// test wrong nodePrivKey
	nodeKeys, privKeys, repoConfig, _ := genKeysAndConfig(t, repoRoot, 2)

	_, err = New(repoConfig.Config, nil, privKeys[0], 0, logger)
	require.NotNil(t, err)

	// test new swarm in direct mode
	nodeKeys, privKeys, repoConfig, _ = genKeysAndConfig(t, repoRoot, 2)

	_, err = New(repoConfig.Config, nodeKeys[0], privKeys[0], 0, logger)
	require.Nil(t, err)

	// test wrong RepoRoot
	nodeKeys, privKeys, repoConfig, _ = genKeysAndConfig(t, repoRoot, 2)

	repoConfig.Config.RepoRoot = "123"
	_, err = New(repoConfig.Config, nodeKeys[0], privKeys[0], 0, logger)
	require.NotNil(t, err)
}

func TestSwarm_Start(t *testing.T) {
	logger := log.NewWithModule("swarm")
	repoRoot, err := ioutil.TempDir("", "node")
	require.Nil(t, err)
	defer os.RemoveAll(repoRoot)

	nodeKeys, privKeys, repoConfig, _ := genKeysAndConfig(t, repoRoot, 2)

	swarm1, err := New(repoConfig.Config, nodeKeys[0], privKeys[0], 0, logger)
	require.Nil(t, err)

	go swarm1.Start()

	swarm2, err := New(repoConfig.Config, nodeKeys[1], privKeys[1], 0, logger)
	require.Nil(t, err)

	go swarm2.Start()

	time.Sleep(time.Second * 6)

	err = swarm1.Stop()
	require.Nil(t, err)
	err = swarm2.Stop()
	require.Nil(t, err)
}

func TestSwarm_Stop_Wrong(t *testing.T) {
	_, _, mockSwarm, _, _, _ := prepare(t)

	// test with no connected peer
	err := mockSwarm.Stop()
	require.NotNil(t, err)
}

func TestSwarm_AsyncSend(t *testing.T) {
	_, _, mockSwarm, mockMsg, mockMultiAddr, mockId := prepare(t)

	// test with wrong id
	err := mockSwarm.AsyncSend("123", mockMsg)
	require.NotNil(t, err)

	// test in right way
	addr, err := AddrToPeerInfo(mockMultiAddr)
	require.Nil(t, err)

	mockSwarm.connectedPeers.Store(mockId, addr)

	err = mockSwarm.AsyncSend(mockId, mockMsg)
	require.Nil(t, err)
}

func TestSwarm_Send(t *testing.T) {
	_, _, mockSwarm, mockMsg, mockMultiAddr, mockId := prepare(t)

	// test with wrong id
	_, err := mockSwarm.Send("123", mockMsg)
	require.NotNil(t, err)

	// test in right way
	addr, err := AddrToPeerInfo(mockMultiAddr)
	require.Nil(t, err)

	mockSwarm.connectedPeers.Store(mockId, addr)

	_, err = mockSwarm.Send(mockId, mockMsg)
	require.Nil(t, err)
}

func TestSwarm_Connect(t *testing.T) {
	_, _, mockSwarm, _, mockMultiAddr, mockId := prepare(t)

	// test with connect error
	addrWrong := &peer2.AddrInfo{
		ID:    "",
		Addrs: nil,
	}
	_, err := mockSwarm.Connect(addrWrong)
	require.NotNil(t, err)

	// test with getRemoteAddress error
	addrWrong = &peer2.AddrInfo{
		ID:    "123",
		Addrs: nil,
	}
	_, err = mockSwarm.Connect(addrWrong)
	require.NotNil(t, err)

	// test in right way
	addr, err := AddrToPeerInfo(mockMultiAddr)
	require.Nil(t, err)
	pierId, err := mockSwarm.Connect(addr)
	require.Nil(t, err)
	require.Equal(t, mockId, pierId)
}

func TestSwarm_SendWithStream(t *testing.T) {
	swarm, _, _, mockMsg, _, _ := prepare(t)
	mockStream := &MockStream{}

	// test with wrong msg
	msg2 := &pb.Message{Type: -1}
	_, err := swarm.SendWithStream(mockStream, msg2)
	require.NotNil(t, err)

	// test in right way
	_, err = swarm.SendWithStream(mockStream, mockMsg)
	require.Nil(t, err)
}

func TestSwarm_AsyncSendWithStream(t *testing.T) {
	swarm, _, _, mockMsg, _, _ := prepare(t)
	mockStream := &MockStream{}

	// test with wrong msg
	msg2 := &pb.Message{Type: -1}
	err := swarm.AsyncSendWithStream(mockStream, msg2)
	require.NotNil(t, err)

	// test in right way
	err = swarm.AsyncSendWithStream(mockStream, mockMsg)
	require.Nil(t, err)
}

func TestSwarm_RegisterMsgHandler(t *testing.T) {
	swarm, _, _, _, _, _ := prepare(t)
	msgCount := 0

	// test with empty handler
	err := swarm.RegisterMsgHandler(pb.Message_APPCHAIN_REGISTER, nil)
	require.NotNil(t, err)

	// test with invalid message type
	err = swarm.RegisterMsgHandler(-1, func(stream network.Stream, message *pb.Message) {
		require.Equal(t, pb.Message_APPCHAIN_REGISTER, message.Type)

		msg := &pb.Message{Type: pb.Message_ACK}
		data, err := msg.Marshal()
		require.Nil(t, err)
		require.Nil(t, stream.AsyncSend(data))
		msgCount++
	})
	require.NotNil(t, err)

	// test with right handler
	err = swarm.RegisterMsgHandler(pb.Message_APPCHAIN_REGISTER, func(stream network.Stream, message *pb.Message) {
		require.Equal(t, pb.Message_APPCHAIN_REGISTER, message.Type)

		msg := &pb.Message{Type: pb.Message_ACK}
		data, err := msg.Marshal()
		require.Nil(t, err)
		require.Nil(t, stream.AsyncSend(data))
		msgCount++
	})
	require.Nil(t, err)
}

func TestSwarm_RegisterMultiMsgHandler(t *testing.T) {
	swarm, _, _, _, _, _ := prepare(t)
	msgCount := 0

	// test with empty handler
	err := swarm.RegisterMultiMsgHandler([]pb.Message_Type{pb.Message_APPCHAIN_REGISTER}, nil)
	require.NotNil(t, err)

	// test in right way
	err = swarm.RegisterMultiMsgHandler([]pb.Message_Type{pb.Message_APPCHAIN_REGISTER}, func(stream network.Stream, message *pb.Message) {
		require.Equal(t, pb.Message_APPCHAIN_REGISTER, message.Type)

		msg := &pb.Message{Type: pb.Message_ACK}
		data, err := msg.Marshal()
		require.Nil(t, err)
		require.Nil(t, stream.AsyncSend(data))
		msgCount++
	})
	require.Nil(t, err)
}

func TestSwarm_RegisterConnectHandler(t *testing.T) {
	swarm, _, _, _, _, _ := prepare(t)

	err := swarm.RegisterConnectHandler(nil)
	require.Nil(t, err)
}

func TestSwarm_FindProviders(t *testing.T) {
	_, _, mockSwarm, _, _, mockId := prepare(t)

	pierId, err := mockSwarm.FindProviders(mockId)
	require.Nil(t, err)
	require.Equal(t, "QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzawe34", pierId)
}

func TestSwarm_Provider(t *testing.T) {
	_, _, mockSwarm, _, _, mockId := prepare(t)

	err := mockSwarm.Provider(mockId, true)
	require.Nil(t, err)
}

func TestSwarm_ConnectedPeerIDs(t *testing.T) {
	_, _, mockSwarm, _, _, _ := prepare(t)
	ids := mockSwarm.ConnectedPeerIDs()
	require.NotNil(t, ids)

	count := mockSwarm.CountConnectedPeers()
	require.NotNil(t, count)
}

func prepare(t *testing.T) (*Swarm, []string, *Swarm, *pb.Message, string, string) {

	repoRoot, err := ioutil.TempDir("", "node")
	require.Nil(t, err)
	defer os.RemoveAll(repoRoot)

	nodeKeys, privKeys, repoConfig, ids := genKeysAndConfig(t, repoRoot, 2)

	swarm, err := New(repoConfig.Config, nodeKeys[0], privKeys[0], 0, log.NewWithModule("swarm"))
	require.Nil(t, err)

	mockMsg := &pb.Message{Type: pb.Message_APPCHAIN_REGISTER}

	mockMultiAddr := "/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64"

	mockId := "QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64"

	mockSwarm := swarm
	mockSwarm.p2p = &MockNetwork{}

	return swarm, ids, mockSwarm, mockMsg, mockMultiAddr, mockId
}
func genKeysAndConfig(t *testing.T, repoRoot string, peerCnt int) ([]crypto.PrivateKey, []crypto.PrivateKey, *repo.Repo, []string) {
	var nodeKeys []crypto.PrivateKey
	var privKeys []crypto.PrivateKey
	var piers []*repo.NetworkPiers
	port := 5001
	var ids []string

	for i := 0; i < peerCnt; i++ {
		var host []string
		key, err := asym.GenerateKeyPair(crypto.ECDSA_P256)
		require.Nil(t, err)
		nodeKeys = append(nodeKeys, key)

		libp2pKey, err := convertToLibp2pPrivKey(key)
		require.Nil(t, err)

		id, err := peer2.IDFromPrivateKey(libp2pKey)
		require.Nil(t, err)
		ids = append(ids, id.String())

		peer := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/", port)
		host = append(host, peer)

		networkPier := &repo.NetworkPiers{
			Pid:   id.String(),
			Hosts: host,
		}
		piers = append(piers, networkPier)

		privKey, err := asym.GenerateKeyPair(crypto.Secp256k1)
		require.Nil(t, err)

		privKeys = append(privKeys, privKey)

		port++
	}

	config := &repo.Config{
		RepoRoot: repoRoot,
	}
	networkConfig := &repo.NetworkConfig{
		Piers: piers,
	}

	repoConfig := &repo.Repo{
		Config:        config,
		NetworkConfig: networkConfig,
	}

	originRoot := "../repo/testdata"
	err := repo.WriteNetworkConfig(originRoot, repoRoot, repoConfig.NetworkConfig)
	require.Nil(t, err)

	return nodeKeys, privKeys, repoConfig, ids
}

//=======================================================================
type MockStream struct {
}

func (ms *MockStream) RemotePeerID() string {
	return ""
}

func (ms *MockStream) RemotePeerAddr() ma.Multiaddr {
	return nil
}

func (ms *MockStream) AsyncSend(data []byte) error {
	msg := &pb.Message{}
	err := msg.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("Unmarshal message: %w", err)
	}

	t := msg.GetType()

	for msgType := range pb.Message_Type_name {
		if msgType == int32(t) {
			return nil
		}
	}
	return fmt.Errorf("AsyncSend: invalid message type")
}

func (ms *MockStream) Send(data []byte) ([]byte, error) {
	msg := &pb.Message{}
	err := msg.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal message: %w", err)
	}

	t := msg.GetType()

	for msgType := range pb.Message_Type_name {
		if msgType == int32(t) {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("Send: invalid message type")
}

func (ms *MockStream) Read(time.Duration) ([]byte, error) {
	return nil, nil
}

//=======================================================================
type MockStreamHandler struct {
}

// get peer new stream true:reusable stream false:non reusable stream
func (msh *MockStreamHandler) GetStream(string, bool) (network.Stream, error) {
	return nil, nil
}

// release stream
func (msh *MockStreamHandler) ReleaseStream(network.Stream) {

}

//=======================================================================
type MockPeerHandler struct {
}

// get local peer id
func (mph *MockPeerHandler) PeerID() string {
	return ""
}

// get peer private key
func (mph *MockPeerHandler) PrivKey() libp2pcry.PrivKey {
	return nil
}

// get peer addr info by peer id
func (mph *MockPeerHandler) PeerInfo(string) (peer2.AddrInfo, error) {
	return peer2.AddrInfo{}, nil
}

// get all network peers
func (mph *MockPeerHandler) GetPeers() []peer2.AddrInfo {
	return nil
}

// get local peer addr
func (mph *MockPeerHandler) LocalAddr() string {
	return ""
}

// get peers num connected
func (mph *MockPeerHandler) PeersNum() int {
	return 0
}

// check if have an open connection to peer
func (mph *MockPeerHandler) IsConnected(peerID string) bool {
	return false
}

// store peer to peer store
func (mph *MockPeerHandler) StorePeer(peer2.AddrInfo) error {
	return nil
}

// GetRemotePubKey gets remote public key
func (mph *MockPeerHandler) GetRemotePubKey(id peer2.ID) (libp2pcry.PubKey, error) {
	return nil, nil
}

//=======================================================================
type MockDHTHandler struct {
}

// searches for a peer with peer id
func (mdhth *MockDHTHandler) FindPeer(string) (peer2.AddrInfo, error) {
	return peer2.AddrInfo{}, nil
}

// Search for peers who are able to provide a given key
//
// When count is 0, this method will return an unbounded number of
// results.
func (mdhth *MockDHTHandler) FindProvidersAsync(id string, count int) (<-chan peer2.AddrInfo, error) {
	if len(id) != 46 {
		return nil, fmt.Errorf("FindProvidersAsync: wrong id %s", id)
	}

	ch := make(chan peer2.AddrInfo)
	addr, err := AddrToPeerInfo("/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzawe34")
	if err != nil {
		return nil, fmt.Errorf("FindProvidersAsync: AddrToPeerInfo wrong")
	}
	go func() {
		ch <- *addr
	}()

	time.Sleep(time.Second)

	return ch, nil
}

// Provide adds the given cid to the content routing system. If 'true' is
// passed, it also announces it, otherwise it is just kept in the local
// accounting of which objects are being provided.
func (mdhth *MockDHTHandler) Provider(string, bool) error {
	return nil
}

//=======================================================================
type MockNetwork struct {
	MockStreamHandler

	MockPeerHandler

	MockDHTHandler
}

// Start start the network service.
func (mn *MockNetwork) Start() error {
	return nil
}

// Stop stop the network service.
func (mn *MockNetwork) Stop() error {
	if mn.PeersNum() == 0 {
		return fmt.Errorf("Stop: there is no connected pier")
	}
	return nil
}

// Connect connects peer by addr.
func (mn *MockNetwork) Connect(addrinfo peer2.AddrInfo) error {
	if addrinfo.ID == "" {
		return fmt.Errorf("Connect: wrong addrinfo %s", addrinfo.String())
	}
	return nil
}

// Disconnect peer with id
func (mn *MockNetwork) Disconnect(string) error {
	return nil
}

// SetConnectionCallback sets the callback after connecting
func (mn *MockNetwork) SetConnectCallback(network.ConnectCallback) {

}

// SetMessageHandler sets message handler
func (mn *MockNetwork) SetMessageHandler(network.MessageHandler) {

}

// AsyncSend sends message to peer with peer id.
func (mn *MockNetwork) AsyncSend(id string, msg []byte) error {
	if len(id) != 46 {
		return fmt.Errorf("AsyncSend: wrong id %s", id)
	}
	return nil
}

// Send sends message to peer with peer id waiting response
func (mn *MockNetwork) Send(id string, data []byte) ([]byte, error) {
	if len(id) != 46 {
		return nil, fmt.Errorf("AsyncSend: wrong id %s", id)
	}

	msg := &pb.Message{}
	if err := msg.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("Unmarshal message: %w", err)
	}

	for msgType := range pb.Message_Type_name {
		if msgType == int32(msg.GetType()) {
			retMsg := Message(pb.Message_ACK, true, []byte(id))
			retData, err := retMsg.Marshal()
			if err != nil {
				return nil, fmt.Errorf("Marshal message: %w", err)
			}
			return retData, nil
		}
	}
	return nil, fmt.Errorf("Send: invalid message type")
}

// Broadcast message to all node
func (mn *MockNetwork) Broadcast([]string, []byte) error {
	return nil
}
