package direct_lite

import "github.com/meshplus/bitxhub-model/pb"

type MockLite struct {
}

func (lite *MockLite) Start() error {
	return nil
}

func (lite *MockLite) Stop() error {
	return nil
}

func (lite *MockLite) QueryHeader(height uint64) (*pb.BlockHeader, error) {
	return nil, nil
}
