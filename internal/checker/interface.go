package checker

import "github.com/meshplus/bitxhub-model/pb"

type Checker interface {
	Check(ibtp *pb.IBTP) (bool, error)
}
