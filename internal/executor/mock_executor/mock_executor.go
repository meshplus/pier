// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package mock_executor is a generated GoMock package.
package mock_executor

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	pb "github.com/meshplus/bitxhub-model/pb"
)

// MockExecutor is a mock of Executor interface
type MockExecutor struct {
	ctrl     *gomock.Controller
	recorder *MockExecutorMockRecorder
}

// MockExecutorMockRecorder is the mock recorder for MockExecutor
type MockExecutorMockRecorder struct {
	mock *MockExecutor
}

// NewMockExecutor creates a new mock instance
func NewMockExecutor(ctrl *gomock.Controller) *MockExecutor {
	mock := &MockExecutor{ctrl: ctrl}
	mock.recorder = &MockExecutorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockExecutor) EXPECT() *MockExecutorMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockExecutor) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockExecutorMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockExecutor)(nil).Start))
}

// Stop mocks base method
func (m *MockExecutor) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockExecutorMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockExecutor)(nil).Stop))
}

// ExecuteIBTP mocks base method
func (m *MockExecutor) ExecuteIBTP(ibtp *pb.IBTP) (*pb.IBTP, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteIBTP", ibtp)
	ret0, _ := ret[0].(*pb.IBTP)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteIBTP indicates an expected call of ExecuteIBTP
func (mr *MockExecutorMockRecorder) ExecuteIBTP(ibtp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteIBTP", reflect.TypeOf((*MockExecutor)(nil).ExecuteIBTP), ibtp)
}

// QueryMeta mocks base method
func (m *MockExecutor) QueryMeta() map[string]uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryMeta")
	ret0, _ := ret[0].(map[string]uint64)
	return ret0
}

// QueryMeta indicates an expected call of QueryMeta
func (mr *MockExecutorMockRecorder) QueryMeta() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryMeta", reflect.TypeOf((*MockExecutor)(nil).QueryMeta))
}

// QueryIBTPReceipt mocks base method
func (m *MockExecutor) QueryIBTPReceipt(from string, index uint64) *pb.IBTP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryIBTPReceipt", from, index, originalIBTP)
	ret0, _ := ret[0].(*pb.IBTP)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryIBTPReceipt indicates an expected call of QueryIBTPReceipt
func (mr *MockExecutorMockRecorder) QueryIBTPReceipt(from, index, originalIBTP interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryIBTPReceipt", reflect.TypeOf((*MockExecutor)(nil).QueryIBTPReceipt), from, index, originalIBTP)
}
