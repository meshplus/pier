package model

import (
	"fmt"
	"github.com/meshplus/bitxhub-model/pb"
)

type PluginResponse struct {
	Status  bool
	Message string
	Result  *pb.IBTP
}

type Payload struct {
	Ok   bool
	Data []byte
}

// WrappedIBTP add IsValid field indicating if this ibtp is valid in bitxhub
type WrappedIBTP struct {
	Ibtp    *pb.IBTP
	IsValid bool
}

func WrapperKey(height uint64) []byte {
	return []byte(fmt.Sprintf("wrapper-%d", height))
}

func IBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("ibtp-%s", id))
}
