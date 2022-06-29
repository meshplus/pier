package txcrypto

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/go-bitxhub-client/mock_client"
	network "github.com/meshplus/go-lightp2p"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestRelayCryptor(t *testing.T) {
	c := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(c)
	loggers.InitializeLogger(repo.DefaultConfig())
	privKey1, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()
	require.Nil(t, err)
	addr2 := address2.String()
	ret := &pb.Receipt{
		Ret: []byte(base64.StdEncoding.EncodeToString(pubBytes2)),
	}

	rc1, err := NewRelayCryptor(mockClient, privKey1, log.NewWithModule("Cryptor"))
	require.Nil(t, err)

	address1, err := privKey1.PublicKey().Address()
	require.Nil(t, err)
	pubBytes1, err := privKey1.PublicKey().Bytes()
	require.Nil(t, err)
	addr1 := address1.String()
	keyMap2 := make(map[string][]byte)
	keyMap2[addr1] = pubBytes1
	rc2 := &RelayCryptor{
		privKey: privKey2,
		keyMap:  keyMap2,
		logger:  log.NewWithModule("Cryptor"),
	}

	mockClient.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), rpcx.String(addr2)).Return(ret, nil).AnyTimes()
	mockClient.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), rpcx.String(addr1)).Return(ret, nil).AnyTimes()
	mockClient.EXPECT().InvokeBVMContract(gomock.Any(), gomock.Any(), gomock.Any(), rpcx.String("")).Return(nil, fmt.Errorf("invoke bvm contract error")).AnyTimes()

	content := []byte("bitxhub cryptor test")
	encryptBytes, err := rc1.Encrypt(content, addr2)
	fmt.Println(err)
	require.Nil(t, err)

	decryptBytes, err := rc2.Decrypt(encryptBytes, addr1)
	require.Nil(t, err)
	require.Equal(t, decryptBytes, content)

	fmt.Println(string(decryptBytes))

	// encrypt with invoke bvm contract error
	_, err = rc1.Encrypt(content, "")
	require.NotNil(t, err)
	// decrypt with wrong pubkey
	rc2.keyMap[addr1] = []byte("")
	_, err = rc2.Decrypt(encryptBytes, addr1)
	require.NotNil(t, err)
}

func TestDirectCryptor(t *testing.T) {
	// todo check err
	c := gomock.NewController(t)

	privKey1, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)
	privKey2, err := asym.GenerateKeyPair(crypto.Secp256k1)
	require.Nil(t, err)

	address2, err := privKey2.PublicKey().Address()
	require.Nil(t, err)
	pubBytes2, err := privKey2.PublicKey().Bytes()

	require.Nil(t, err)
	addr2 := address2.String()
	mockPeerMgr := mock_peermgr.NewMockPeerManager(c)
	mockPeerMgr.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	dc1, err := NewDirectCryptor(mockPeerMgr, privKey1, log.NewWithModule("Cryptor"))
	require.Nil(t, err)

	address1, err := privKey1.PublicKey().Address()
	require.Nil(t, err)
	pubBytes1, err := privKey1.PublicKey().Bytes()
	require.Nil(t, err)
	addr1 := address1.String()
	keyMap2 := make(map[string][]byte)
	keyMap2[addr1] = pubBytes1
	dc2 := &DirectCryptor{
		peerMgr: mockPeerMgr,
		privKey: privKey2,
		keyMap:  keyMap2,
		logger:  log.NewWithModule("Cryptor"),
	}

	msgPub1 := &pb.Message{
		Type:    pb.Message_PUBKEY_GET_ACK,
		Data:    pubBytes1,
		Version: nil,
	}

	msgPub2 := &pb.Message{
		Type:    pb.Message_PUBKEY_GET_ACK,
		Data:    pubBytes2,
		Version: nil,
	}
	mockPeerMgr.EXPECT().AsyncSendWithStream(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockPeerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(addr string, msg *pb.Message) (*pb.Message, error) {
		var stream network.Stream
		if addr == addr2 {
			dc2.handleGetPublicKey(stream, msg)
			return msgPub2, nil
		}

		if addr == addr1 {
			dc1.(*DirectCryptor).handleGetPublicKey(stream, msg)
			return msgPub1, nil
		}

		return nil, fmt.Errorf("handle get public key err")
	}).Times(1)
	content := []byte("bitxhub cryptor test")
	encryptBytes, err := dc1.Encrypt(content, addr2)
	require.Nil(t, err)

	decryptBytes, err := dc2.Decrypt(encryptBytes, addr1)
	require.Nil(t, err)
	require.Equal(t, decryptBytes, content)

	fmt.Println(string(decryptBytes))

	// encrypt with nil address
	AddrErr := fmt.Errorf("wrong addr")
	mockPeerMgr.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil, AddrErr).Times(1)
	_, err = dc1.Encrypt(content, "")
	require.NotNil(t, err)
	require.Equal(t, true, strings.Contains(err.Error(), AddrErr.Error()))

	// decrypt with wrong pubkey
	dc2.keyMap[addr1] = []byte("")
	_, err = dc2.Decrypt(encryptBytes, addr1)
	require.NotNil(t, err)
}
