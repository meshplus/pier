package peermgr

import (
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym/ecdsa"
	peermgr "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestSwarm_Start(t *testing.T) {
	keys, config, ids := genKeysAndConfig(t, 2)

	swarm1, err := New(config, keys[0])
	require.Nil(t, err)

	require.Nil(t, swarm1.Start())
	time.Sleep(time.Second)

	swarm2, err := New(config, keys[1])
	require.Nil(t, err)

	require.Nil(t, swarm2.Start())

	time.Sleep(time.Second * 2)

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

	msg := &peermgr.Message{Type: peermgr.Message_APPCHAIN_REGISTER}
	msg2, err := swarm2.Send(ids[0], msg)
	require.Nil(t, err)
	require.Equal(t, peermgr.Message_ACK, msg2.Type)

	msg = &peermgr.Message{Type: peermgr.Message_IBTP_GET}
	require.Nil(t, swarm1.AsyncSend(ids[1], msg))

	msg2, err = swarm1.Send("123", msg)
	require.NotNil(t, err)
	require.Nil(t, msg2)

	time.Sleep(time.Second)
	require.Equal(t, 2, msgCount)
}

func TestSwarm_Stop(t *testing.T) {
	keys, config, ids := genKeysAndConfig(t, 2)

	swarm1, err := New(config, keys[0])
	require.Nil(t, err)

	require.Nil(t, swarm1.Start())
	time.Sleep(time.Second)

	swarm2, err := New(config, keys[1])
	require.Nil(t, err)

	require.Nil(t, swarm2.Start())

	time.Sleep(time.Second * 2)

	msgCount := 0
	err = swarm1.RegisterMsgHandler(peermgr.Message_IBTP_GET, func(stream network.Stream, message *peermgr.Message) {
		require.Equal(t, peermgr.Message_IBTP_GET, message.Type)

		msg := &peermgr.Message{Type: peermgr.Message_ACK}
		require.Nil(t, swarm1.SendWithStream(stream, msg))
		msgCount++
	})
	require.Nil(t, err)

	require.Nil(t, swarm1.Stop())
	time.Sleep(time.Second)

	msg := &peermgr.Message{Type: peermgr.Message_IBTP_GET}
	_, err = swarm2.Send(ids[0], msg)
	require.NotNil(t, err)

	swarm1, err = New(config, keys[0])
	require.Nil(t, err)

	require.Nil(t, swarm1.Start())

	err = swarm1.RegisterMsgHandler(peermgr.Message_IBTP_GET, func(stream network.Stream, message *peermgr.Message) {
		require.Equal(t, peermgr.Message_IBTP_GET, message.Type)

		msg := &peermgr.Message{Type: peermgr.Message_ACK}
		err := swarm1.SendWithStream(stream, msg)
		require.Nil(t, err)
		msgCount++
	})
	require.Nil(t, err)

	time.Sleep(time.Second * 3)

	msg = &peermgr.Message{Type: peermgr.Message_IBTP_GET}
	msg2, err := swarm2.Send(ids[0], msg)
	require.Nil(t, err)
	require.Equal(t, peermgr.Message_ACK, msg2.Type)
}

func genKeysAndConfig(t *testing.T, peerCnt int) ([]crypto.PrivateKey, *repo.Config, []string) {
	var keys []crypto.PrivateKey
	var peers []string
	var ids []string
	port := 5001

	for i := 0; i < peerCnt; i++ {
		key, err := ecdsa.GenerateKey(ecdsa.Secp256r1)
		require.Nil(t, err)
		keys = append(keys, key)
		addr, err := key.PublicKey().Address()
		require.Nil(t, err)
		ids = append(ids, addr.String())

		libp2pKey, err := convertToLibp2pPrivKey(key)
		require.Nil(t, err)

		id, err := peer2.IDFromPrivateKey(libp2pKey)
		require.Nil(t, err)

		peer := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, id)
		peers = append(peers, peer)
		port++
	}

	config := &repo.Config{}
	config.Mode.Direct.Peers = peers

	return keys, config, ids
}
