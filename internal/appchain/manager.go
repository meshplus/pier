package appchain

import (
	"encoding/json"

	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/sirupsen/logrus"
)

var (
	logger                       = log.NewWithModule("appchain_mgr")
	_      appchainmgr.Persister = (*Persister)(nil)
)

type Persister struct {
	addr    string
	storage storage.Storage
}

type Manager struct {
	PeerManager peermgr.PeerManager
	Mgr         appchainmgr.AppchainMgr
}

func (m Persister) Caller() string {
	return m.addr
}

func (m Persister) Logger() logrus.FieldLogger {
	return logger
}

func (m Persister) Has(key string) bool {
	ok, _ := m.storage.Has([]byte(key))
	return ok
}

func (m Persister) Get(key string) (bool, []byte) {
	data, err := m.storage.Get([]byte(key))
	if err != nil {
		return false, nil
	}
	return true, data
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
	err := m.storage.Put([]byte(key), value)
	if err != nil {
		panic(err.Error())
	}
}

func (m Persister) SetObject(key string, value interface{}) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err.Error())
	}
	m.Set(key, data)
}

func (m Persister) Delete(key string) {
	err := m.storage.Delete([]byte(key))
	if err != nil {
		panic(err.Error())
	}
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

func NewManager(addr string, storage storage.Storage, pm peermgr.PeerManager) (*Manager, error) {
	appchainMgr := appchainmgr.New(&Persister{addr: addr, storage: storage})
	am := &Manager{
		PeerManager: pm,
		Mgr:         appchainMgr,
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
