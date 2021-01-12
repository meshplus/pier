package appchain

import (
	"encoding/json"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/sirupsen/logrus"
)

var _ appchainmgr.Persister = (*Persister)(nil)

type Persister struct {
	addr    string
	storage storage.Storage
	logger  logrus.FieldLogger
}

type Manager struct {
	logger      logrus.FieldLogger
	PeerManager peermgr.PeerManager
	Mgr         appchainmgr.AppchainMgr
}

func (m Persister) Caller() string {
	return m.addr
}

func (m Persister) Logger() logrus.FieldLogger {
	return m.logger
}

func (m Persister) Has(key string) bool {
	return m.storage.Has([]byte(key))
}

func (m Persister) Get(key string) (bool, []byte) {
	return true, m.storage.Get([]byte(key))
}

func (m Persister) GetObject(key string, ret interface{}) bool {
	ok, data := m.Get(key)
	if !ok {
		return false
	}
	err := json.Unmarshal(data, ret)
	return err == nil
}

func (m Persister) Set(key string, value []byte) {
	m.storage.Put([]byte(key), value)
}

func (m Persister) SetObject(key string, value interface{}) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err.Error())
	}
	m.Set(key, data)
}

func (m Persister) Delete(key string) {
	m.storage.Delete([]byte(key))
}

func (m Persister) Query(prefix string) (bool, [][]byte) {
	var ret [][]byte
	it := m.storage.Prefix([]byte(prefix))
	for it.Next() {
		val := make([]byte, len(it.Value()))
		copy(val, it.Value())
		ret = append(ret, val)
	}
	return len(ret) != 0, ret
}

func NewManager(addr string, storage storage.Storage, pm peermgr.PeerManager, logger logrus.FieldLogger) (*Manager, error) {
	appchainMgr := appchainmgr.New(&Persister{addr: addr, storage: storage, logger: logger})
	am := &Manager{
		PeerManager: pm,
		Mgr:         appchainMgr,
		logger:      logger,
	}

	err := pm.RegisterMultiMsgHandler([]peerproto.Message_Type{
		peerproto.Message_APPCHAIN_REGISTER,
		peerproto.Message_APPCHAIN_UPDATE,
		peerproto.Message_APPCHAIN_GET,
	}, am.handleMessage)
	if err != nil {
		return nil, err
	}

	return am, nil
}
