package peermgr

import peerproto "github.com/meshplus/pier/internal/peermgr/proto"

const (
	V1 = "1.0"
)

func Message(typ peerproto.Message_Type, ok bool, data []byte) *peerproto.Message {
	return &peerproto.Message{
		Type:    typ,
		Version: V1,
		Payload: &peerproto.Payload{
			Ok:   ok,
			Data: data,
		},
	}
}
