package peermgr

import (
	"github.com/meshplus/bitxhub-model/pb"
)

const (
	V1 = "1.0"
)

func Message(typ pb.Message_Type, ok bool, data []byte) *pb.Message {
	payload := &pb.PierPayload{Ok: ok, Data: data}
	mData, err := payload.Marshal()
	if err != nil {
		return nil
	}
	//TODO: Marshal error handing.
	if err != nil {
		return nil
	}
	return &pb.Message{
		Type:    typ,
		Version: []byte(V1),
		Data:    mData,
	}
}

func DataToPayload(msg *pb.Message) *pb.PierPayload {
	payload := &pb.PierPayload{}
	err := payload.Unmarshal(msg.Data)
	if err != nil {
		return nil
	}
	return payload
}
