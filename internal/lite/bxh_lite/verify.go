package bxh_lite

import (
	"github.com/meshplus/bitxhub-model/pb"
)

func (lite *BxhLite) verifyHeader(h *pb.BlockHeader) (bool, error) {
	// TODO: blocked by signature mechanism implementation of BitXHub
	return true, nil
}
