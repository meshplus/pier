package peermgr

import (
	"encoding/json"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
)

const (
	V1 = "1.0"
)

func MessageWithPayload(typ pb.Message_Type, pd *model.Payload) *pb.Message {
	mData, err := json.Marshal(pd)
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

func Message(typ pb.Message_Type, ok bool, data []byte) *pb.Message {
	payload := &model.Payload{Ok: ok, Data: data}
	mData, err := json.Marshal(payload)
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

func DataToPayload(msg *pb.Message) *model.Payload {
	payload := &model.Payload{}
	if err := json.Unmarshal(msg.Data, payload); err != nil {
		//TODO: Unmarshal error handing.
		return nil
	}
	return payload
}
