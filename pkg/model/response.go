package model

import "github.com/meshplus/bitxhub-model/pb"

type PluginResponse struct {
	Status  bool
	Message string
	Result  *pb.IBTP
}
