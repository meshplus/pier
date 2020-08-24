package peermgr

import (
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	peermgr "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestSwarm_Start(t *testing.T) {
	nodeKeys, privKeys, config := genKeysAndConfig(t, 2)

	swarm1, err := New(config, nodeKeys[0], privKeys[0])
	require.Nil(t, err)

	require.Nil(t, swarm1.Start())
	time.Sleep(time.Second * 2)

	swarm2, err := New(config, nodeKeys[1], privKeys[1])
	require.Nil(t, err)

	require.Nil(t, swarm2.Start())

	time.Sleep(time.Second * 6)

	msgCount := 0
	err = swarm1.RegisterMsgHandler(peermgr.Message_APPCHAIN_REGISTER, func(stream network.Stream, message *peermgr.Message) {
		require.Equal(t, peermgr.Message_APPCHAIN_REGISTER, message.Type)

		msg := &peermgr.Message{Type: peermgr.Message_ACK}
		require.Nil(t, swarm1.SendWithStream(stream, msg))
		msgCount++
	})
	require.Nil(t, err)

	err = swarm2.RegisterMsgHandler(peermgr.Message_IBTP_GET, func(stream network.Stream, message *peermgr.Message) {
		require.Equal(t, peermgr.Message_IBTP_GET, message.Type)
		msgCount++
	})
	require.Nil(t, err)

	// TODO
	//msg := &peermgr.Message{Type: peermgr.Message_APPCHAIN_REGISTER}
	//msg2, err := swarm2.Send(ids[0], msg)
	//require.Nil(t, err)
	//require.Equal(t, peermgr.Message_ACK, msg2.Type)
	//
	//msg = &peermgr.Message{Type: peermgr.Message_IBTP_GET}
	//require.Nil(t, swarm1.AsyncSend(ids[1], msg))
	//
	//msg2, err = swarm1.Send("123", msg)
	//require.NotNil(t, err)
	//require.Nil(t, msg2)
	//
	//time.Sleep(time.Second)
	//require.Equal(t, 2, msgCount)
}

func genKeysAndConfig(t *testing.T, peerCnt int) ([]crypto.PrivateKey, []crypto.PrivateKey, *repo.Config) {
	var nodeKeys []crypto.PrivateKey
	var privKeys []crypto.PrivateKey
	var peers []string
	port := 5001

	for i := 0; i < peerCnt; i++ {
		key, err := asym.GenerateKeyPair(crypto.ECDSA_P256)
		require.Nil(t, err)
		nodeKeys = append(nodeKeys, key)

		libp2pKey, err := convertToLibp2pPrivKey(key)
		require.Nil(t, err)

		id, err := peer2.IDFromPrivateKey(libp2pKey)
		require.Nil(t, err)

		peer := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, id)
		peers = append(peers, peer)

		privKey, err := asym.GenerateKeyPair(crypto.Secp256k1)
		require.Nil(t, err)

		privKeys = append(privKeys, privKey)

		port++
	}

	config := &repo.Config{}
	config.Mode.Direct.Peers = peers

	return nodeKeys, privKeys, config
}
