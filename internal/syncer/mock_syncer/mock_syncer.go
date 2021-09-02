// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package mock_syncer is a generated GoMock package.
package mock_syncer

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	pb "github.com/meshplus/bitxhub-model/pb"
	syncer "github.com/meshplus/pier/internal/syncer"
	model "github.com/meshplus/pier/pkg/model"
)

// MockSyncer is a mock of Syncer interface.
type MockSyncer struct {
	ctrl     *gomock.Controller
	recorder *MockSyncerMockRecorder
}

// MockSyncerMockRecorder is the mock recorder for MockSyncer.
type MockSyncerMockRecorder struct {
	mock *MockSyncer
}

// NewMockSyncer creates a new mock instance.
func NewMockSyncer(ctrl *gomock.Controller) *MockSyncer {
	mock := &MockSyncer{ctrl: ctrl}
	mock.recorder = &MockSyncerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncer) EXPECT() *MockSyncerMockRecorder {
	return m.recorder
}

// GetAssetExchangeSigns mocks base method.
func (m *MockSyncer) GetAssetExchangeSigns(id string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAssetExchangeSigns", id)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAssetExchangeSigns indicates an expected call of GetAssetExchangeSigns.
func (mr *MockSyncerMockRecorder) GetAssetExchangeSigns(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAssetExchangeSigns", reflect.TypeOf((*MockSyncer)(nil).GetAssetExchangeSigns), id)
}

// GetBitXHubIDs mocks base method.
func (m *MockSyncer) GetBitXHubIDs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBitXHubIDs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBitXHubIDs indicates an expected call of GetBitXHubIDs.
func (mr *MockSyncerMockRecorder) GetBitXHubIDs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBitXHubIDs", reflect.TypeOf((*MockSyncer)(nil).GetBitXHubIDs))
}

// GetChainID mocks base method.
func (m *MockSyncer) GetChainID() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChainID")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChainID indicates an expected call of GetChainID.
func (mr *MockSyncerMockRecorder) GetChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChainID", reflect.TypeOf((*MockSyncer)(nil).GetChainID))
}

// GetIBTPSigns mocks base method.
func (m *MockSyncer) GetIBTPSigns(ibtp *pb.IBTP) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetIBTPSigns", ibtp)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetIBTPSigns indicates an expected call of GetIBTPSigns.
func (mr *MockSyncerMockRecorder) GetIBTPSigns(ibtp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetIBTPSigns", reflect.TypeOf((*MockSyncer)(nil).GetIBTPSigns), ibtp)
}

// GetServiceIDs mocks base method.
func (m *MockSyncer) GetServiceIDs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetServiceIDs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetServiceIDs indicates an expected call of GetServiceIDs.
func (mr *MockSyncerMockRecorder) GetServiceIDs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetServiceIDs", reflect.TypeOf((*MockSyncer)(nil).GetServiceIDs))
}

// GetTxStatus mocks base method.
func (m *MockSyncer) GetTxStatus(id string) (pb.TransactionStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTxStatus", id)
	ret0, _ := ret[0].(pb.TransactionStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTxStatus indicates an expected call of GetTxStatus.
func (mr *MockSyncerMockRecorder) GetTxStatus(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTxStatus", reflect.TypeOf((*MockSyncer)(nil).GetTxStatus), id)
}

// ListenIBTP mocks base method.
func (m *MockSyncer) ListenIBTP() <-chan *model.WrappedIBTP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListenIBTP")
	ret0, _ := ret[0].(<-chan *model.WrappedIBTP)
	return ret0
}

// ListenIBTP indicates an expected call of ListenIBTP.
func (mr *MockSyncerMockRecorder) ListenIBTP() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListenIBTP", reflect.TypeOf((*MockSyncer)(nil).ListenIBTP))
}

// QueryIBTP mocks base method.
func (m *MockSyncer) QueryIBTP(ibtpID string, isReq bool) (*pb.IBTP, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryIBTP", ibtpID, isReq)
	ret0, _ := ret[0].(*pb.IBTP)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// QueryIBTP indicates an expected call of QueryIBTP.
func (mr *MockSyncerMockRecorder) QueryIBTP(ibtpID, isReq interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryIBTP", reflect.TypeOf((*MockSyncer)(nil).QueryIBTP), ibtpID, isReq)
}

// QueryInterchainMeta mocks base method.
func (m *MockSyncer) QueryInterchainMeta(serviceID string) *pb.Interchain {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryInterchainMeta", serviceID)
	ret0, _ := ret[0].(*pb.Interchain)
	return ret0
}

// QueryInterchainMeta indicates an expected call of QueryInterchainMeta.
func (mr *MockSyncerMockRecorder) QueryInterchainMeta(serviceID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryInterchainMeta", reflect.TypeOf((*MockSyncer)(nil).QueryInterchainMeta), serviceID)
}

// RegisterRollbackHandler mocks base method.
func (m *MockSyncer) RegisterRollbackHandler(handler syncer.RollbackHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterRollbackHandler", handler)
	ret0, _ := ret[0].(error)
	return ret0
}

// RegisterRollbackHandler indicates an expected call of RegisterRollbackHandler.
func (mr *MockSyncerMockRecorder) RegisterRollbackHandler(handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterRollbackHandler", reflect.TypeOf((*MockSyncer)(nil).RegisterRollbackHandler), handler)
}

// SendIBTP mocks base method.
func (m *MockSyncer) SendIBTP(ibtp *pb.IBTP) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendIBTP", ibtp)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendIBTP indicates an expected call of SendIBTP.
func (mr *MockSyncerMockRecorder) SendIBTP(ibtp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendIBTP", reflect.TypeOf((*MockSyncer)(nil).SendIBTP), ibtp)
}

// SendIBTPWithRetry mocks base method.
func (m *MockSyncer) SendIBTPWithRetry(ibtp *pb.IBTP) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SendIBTPWithRetry", ibtp)
}

// SendIBTPWithRetry indicates an expected call of SendIBTPWithRetry.
func (mr *MockSyncerMockRecorder) SendIBTPWithRetry(ibtp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendIBTPWithRetry", reflect.TypeOf((*MockSyncer)(nil).SendIBTPWithRetry), ibtp)
}

// Start mocks base method.
func (m *MockSyncer) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockSyncerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockSyncer)(nil).Start))
}

// Stop mocks base method.
func (m *MockSyncer) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockSyncerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockSyncer)(nil).Stop))
}
