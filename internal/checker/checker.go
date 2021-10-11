package checker

import (
	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_checker/mock_checker.go -package mock_checker -source checker.go
type Checker interface {
	BasicCheck(ibtp *pb.IBTP) (bool, error)

	CheckProof(ibtp *pb.IBTP) error
}
