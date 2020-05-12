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
	"github.com/stretchr/testify/assert"
)

func TestSwarm_Start(t *testing.T) {
	keys, config, ids := genKeysAndConfig(t, 2)

	swarm1, err := New(config, keys[0])
	assert.Nil(t, err)

	swarm2, err := New(config, keys[1])
	assert.Nil(t, err)

	err = swarm1.Start()
	assert.Nil(t, err)

	err = swarm2.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second)

	msgCount := 0
	err = swarm1.RegisterMsgHandler(peermgr.Message_APPCHAIN, func(stream network.Stream, message *peermgr.Message) {
		assert.Equal(t, peermgr.Message_APPCHAIN, message.Type)

		msg := &peermgr.Message{Type: peermgr.Message_APPCHAIN_ACK}
		err := swarm1.SendWithStream(stream, msg)
		assert.Nil(t, err)
		msgCount++
	})
	assert.Nil(t, err)

	err = swarm2.RegisterMsgHandler(peermgr.Message_IBTP, func(stream network.Stream, message *peermgr.Message) {
		assert.Equal(t, peermgr.Message_IBTP, message.Type)
		msgCount++
	})
	assert.Nil(t, err)

	msg := &peermgr.Message{Type: peermgr.Message_APPCHAIN}
	msg2, err := swarm2.Send(ids[0], msg)
	assert.Nil(t, err)
	assert.Equal(t, peermgr.Message_APPCHAIN_ACK, msg2.Type)

	msg = &peermgr.Message{Type: peermgr.Message_IBTP}
	err = swarm1.AsyncSend(ids[1], msg)
	assert.Nil(t, err)

	msg2, err = swarm1.Send("123", msg)
	assert.NotNil(t, err)
	assert.Nil(t, msg2)

	time.Sleep(time.Second)
	assert.Equal(t, 2, msgCount)
}

func genKeysAndConfig(t *testing.T, peerCnt int) ([]crypto.PrivateKey, *repo.Config, []string) {
	var keys []crypto.PrivateKey
	var peers []string
	var ids []string
	port := 5001

	for i := 0; i < peerCnt; i++ {
		key, err := ecdsa.GenerateKey(ecdsa.Secp256r1)
		assert.Nil(t, err)
		keys = append(keys, key)
		addr, err := key.PublicKey().Address()
		assert.Nil(t, err)
		ids = append(ids, addr.String())

		libp2pKey, err := convertToLibp2pPrivKey(key)
		assert.Nil(t, err)

		id, err := peer2.IDFromPrivateKey(libp2pKey)
		assert.Nil(t, err)

		peer := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, id)
		peers = append(peers, peer)
		port++
	}

	config := &repo.Config{}
	config.Mode.Direct.Peers = peers

	return keys, config, ids
}
