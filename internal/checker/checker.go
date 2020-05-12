package checker

import "github.com/meshplus/bitxhub-model/pb"

type MockChecker struct {
}

func (ck *MockChecker) Check(ibtp *pb.IBTP) (bool, error) {
	return true, nil
}
