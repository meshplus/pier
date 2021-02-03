// Code generated by MockGen. DO NOT EDIT.
// Source: peermgr.go

// Package mock_peermgr is a generated GoMock package.
package mock_peermgr

import (
	gomock "github.com/golang/mock/gomock"
	peer "github.com/libp2p/go-libp2p-core/peer"
	network "github.com/meshplus/go-lightp2p"
	peermgr "github.com/meshplus/pier/internal/peermgr"
	peermgr0 "github.com/meshplus/pier/internal/peermgr/proto"
	reflect "reflect"
)

// MockPeerManager is a mock of PeerManager interface
type MockPeerManager struct {
	ctrl     *gomock.Controller
	recorder *MockPeerManagerMockRecorder
}

// MockPeerManagerMockRecorder is the mock recorder for MockPeerManager
type MockPeerManagerMockRecorder struct {
	mock *MockPeerManager
}

// NewMockPeerManager creates a new mock instance
func NewMockPeerManager(ctrl *gomock.Controller) *MockPeerManager {
	mock := &MockPeerManager{ctrl: ctrl}
	mock.recorder = &MockPeerManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPeerManager) EXPECT() *MockPeerManagerMockRecorder {
	return m.recorder
}

// FindProviders mocks base method
func (m *MockPeerManager) FindProviders(id string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindProviders", id)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindProviders indicates an expected call of FindProviders
func (mr *MockPeerManagerMockRecorder) FindProviders(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindProviders", reflect.TypeOf((*MockPeerManager)(nil).FindProviders), id)
}

// Provider mocks base method
func (m *MockPeerManager) Provider(arg0 string, arg1 bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Provider", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Provider indicates an expected call of Provider
func (mr *MockPeerManagerMockRecorder) Provider(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Provider", reflect.TypeOf((*MockPeerManager)(nil).Provider), arg0, arg1)
}

// Start mocks base method
func (m *MockPeerManager) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockPeerManagerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockPeerManager)(nil).Start))
}

// Stop mocks base method
func (m *MockPeerManager) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockPeerManagerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockPeerManager)(nil).Stop))
}

// AsyncSend mocks base method
func (m *MockPeerManager) AsyncSend(arg0 string, arg1 *peermgr0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AsyncSend", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AsyncSend indicates an expected call of AsyncSend
func (mr *MockPeerManagerMockRecorder) AsyncSend(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AsyncSend", reflect.TypeOf((*MockPeerManager)(nil).AsyncSend), arg0, arg1)
}

// Connect mocks base method
func (m *MockPeerManager) Connect(info *peer.AddrInfo) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Connect", info)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Connect indicates an expected call of Connect
func (mr *MockPeerManagerMockRecorder) Connect(info interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connect", reflect.TypeOf((*MockPeerManager)(nil).Connect), info)
}

// SendWithStream mocks base method
func (m *MockPeerManager) SendWithStream(arg0 network.Stream, arg1 *peermgr0.Message) (*peermgr0.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendWithStream", arg0, arg1)
	ret0, _ := ret[0].(*peermgr0.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendWithStream indicates an expected call of SendWithStream
func (mr *MockPeerManagerMockRecorder) SendWithStream(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendWithStream", reflect.TypeOf((*MockPeerManager)(nil).SendWithStream), arg0, arg1)
}

// AsyncSendWithStream mocks base method
func (m *MockPeerManager) AsyncSendWithStream(arg0 network.Stream, arg1 *peermgr0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AsyncSendWithStream", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AsyncSendWithStream indicates an expected call of AsyncSendWithStream
func (mr *MockPeerManagerMockRecorder) AsyncSendWithStream(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AsyncSendWithStream", reflect.TypeOf((*MockPeerManager)(nil).AsyncSendWithStream), arg0, arg1)
}

// Send mocks base method
func (m *MockPeerManager) Send(arg0 string, arg1 *peermgr0.Message) (*peermgr0.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0, arg1)
	ret0, _ := ret[0].(*peermgr0.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Send indicates an expected call of Send
func (mr *MockPeerManagerMockRecorder) Send(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockPeerManager)(nil).Send), arg0, arg1)
}

// Peers mocks base method
func (m *MockPeerManager) Peers() map[string]*peer.AddrInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Peers")
	ret0, _ := ret[0].(map[string]*peer.AddrInfo)
	return ret0
}

// Peers indicates an expected call of Peers
func (mr *MockPeerManagerMockRecorder) Peers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Peers", reflect.TypeOf((*MockPeerManager)(nil).Peers))
}

// RegisterMsgHandler mocks base method
func (m *MockPeerManager) RegisterMsgHandler(arg0 peermgr0.Message_Type, arg1 peermgr.MessageHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterMsgHandler", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RegisterMsgHandler indicates an expected call of RegisterMsgHandler
func (mr *MockPeerManagerMockRecorder) RegisterMsgHandler(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterMsgHandler", reflect.TypeOf((*MockPeerManager)(nil).RegisterMsgHandler), arg0, arg1)
}

// RegisterMultiMsgHandler mocks base method
func (m *MockPeerManager) RegisterMultiMsgHandler(arg0 []peermgr0.Message_Type, arg1 peermgr.MessageHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterMultiMsgHandler", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RegisterMultiMsgHandler indicates an expected call of RegisterMultiMsgHandler
func (mr *MockPeerManagerMockRecorder) RegisterMultiMsgHandler(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterMultiMsgHandler", reflect.TypeOf((*MockPeerManager)(nil).RegisterMultiMsgHandler), arg0, arg1)
}

// RegisterConnectHandler mocks base method
func (m *MockPeerManager) RegisterConnectHandler(arg0 peermgr.ConnectHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterConnectHandler", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RegisterConnectHandler indicates an expected call of RegisterConnectHandler
func (mr *MockPeerManagerMockRecorder) RegisterConnectHandler(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterConnectHandler", reflect.TypeOf((*MockPeerManager)(nil).RegisterConnectHandler), arg0)
}

// MockDHTManager is a mock of DHTManager interface
type MockDHTManager struct {
	ctrl     *gomock.Controller
	recorder *MockDHTManagerMockRecorder
}

// MockDHTManagerMockRecorder is the mock recorder for MockDHTManager
type MockDHTManagerMockRecorder struct {
	mock *MockDHTManager
}

// NewMockDHTManager creates a new mock instance
func NewMockDHTManager(ctrl *gomock.Controller) *MockDHTManager {
	mock := &MockDHTManager{ctrl: ctrl}
	mock.recorder = &MockDHTManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockDHTManager) EXPECT() *MockDHTManagerMockRecorder {
	return m.recorder
}

// FindProviders mocks base method
func (m *MockDHTManager) FindProviders(id string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindProviders", id)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindProviders indicates an expected call of FindProviders
func (mr *MockDHTManagerMockRecorder) FindProviders(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindProviders", reflect.TypeOf((*MockDHTManager)(nil).FindProviders), id)
}

// Provider mocks base method
func (m *MockDHTManager) Provider(arg0 string, arg1 bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Provider", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Provider indicates an expected call of Provider
func (mr *MockDHTManagerMockRecorder) Provider(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Provider", reflect.TypeOf((*MockDHTManager)(nil).Provider), arg0, arg1)
}
