// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: message.proto

package peermgr

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Message_Type int32

const (
	Message_APPCHAIN_REGISTER        Message_Type = 0
	Message_APPCHAIN_UPDATE          Message_Type = 1
	Message_APPCHAIN_GET             Message_Type = 2
	Message_INTERCHAIN_META_GET      Message_Type = 3
	Message_RULE_DEPLOY              Message_Type = 4
	Message_IBTP_GET                 Message_Type = 5
	Message_IBTP_SEND                Message_Type = 6
	Message_IBTP_RECEIPT_SEND        Message_Type = 7
	Message_ROUTER_IBTP_SEND         Message_Type = 8
	Message_ROUTER_IBTP_RECEIPT_SEND Message_Type = 9
	Message_ADDRESS_GET              Message_Type = 10
	Message_ROUTER_INTERCHAIN_SEND   Message_Type = 11
	Message_ACK                      Message_Type = 12
	Message_DATA_GET                 Message_Type = 13
)

var Message_Type_name = map[int32]string{
	0:  "APPCHAIN_REGISTER",
	1:  "APPCHAIN_UPDATE",
	2:  "APPCHAIN_GET",
	3:  "INTERCHAIN_META_GET",
	4:  "RULE_DEPLOY",
	5:  "IBTP_GET",
	6:  "IBTP_SEND",
	7:  "IBTP_RECEIPT_SEND",
	8:  "ROUTER_IBTP_SEND",
	9:  "ROUTER_IBTP_RECEIPT_SEND",
	10: "ADDRESS_GET",
	11: "ROUTER_INTERCHAIN_SEND",
	12: "ACK",
	13: "DATA_GET",
}

var Message_Type_value = map[string]int32{
	"APPCHAIN_REGISTER":        0,
	"APPCHAIN_UPDATE":          1,
	"APPCHAIN_GET":             2,
	"INTERCHAIN_META_GET":      3,
	"RULE_DEPLOY":              4,
	"IBTP_GET":                 5,
	"IBTP_SEND":                6,
	"IBTP_RECEIPT_SEND":        7,
	"ROUTER_IBTP_SEND":         8,
	"ROUTER_IBTP_RECEIPT_SEND": 9,
	"ADDRESS_GET":              10,
	"ROUTER_INTERCHAIN_SEND":   11,
	"ACK":                      12,
	"DATA_GET":                 13,
}

func (x Message_Type) String() string {
	return proto.EnumName(Message_Type_name, int32(x))
}

func (Message_Type) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_33c57e4bae7b9afd, []int{0, 0}
}

type Message struct {
	Type    Message_Type `protobuf:"varint,1,opt,name=type,proto3,enum=peermgr.Message_Type" json:"type,omitempty"`
	Payload *Payload     `protobuf:"bytes,2,opt,name=payload,proto3" json:"payload,omitempty"`
	Version string       `protobuf:"bytes,3,opt,name=version,proto3" json:"version,omitempty"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_33c57e4bae7b9afd, []int{0}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Message.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(m, src)
}
func (m *Message) XXX_Size() int {
	return m.Size()
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetType() Message_Type {
	if m != nil {
		return m.Type
	}
	return Message_APPCHAIN_REGISTER
}

func (m *Message) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *Message) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

type Payload struct {
	Ok   bool   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (m *Payload) Reset()         { *m = Payload{} }
func (m *Payload) String() string { return proto.CompactTextString(m) }
func (*Payload) ProtoMessage()    {}
func (*Payload) Descriptor() ([]byte, []int) {
	return fileDescriptor_33c57e4bae7b9afd, []int{1}
}
func (m *Payload) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Payload) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Payload.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Payload) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Payload.Merge(m, src)
}
func (m *Payload) XXX_Size() int {
	return m.Size()
}
func (m *Payload) XXX_DiscardUnknown() {
	xxx_messageInfo_Payload.DiscardUnknown(m)
}

var xxx_messageInfo_Payload proto.InternalMessageInfo

func (m *Payload) GetOk() bool {
	if m != nil {
		return m.Ok
	}
	return false
}

func (m *Payload) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterEnum("peermgr.Message_Type", Message_Type_name, Message_Type_value)
	proto.RegisterType((*Message)(nil), "peermgr.Message")
	proto.RegisterType((*Payload)(nil), "peermgr.Payload")
}

func init() { proto.RegisterFile("message.proto", fileDescriptor_33c57e4bae7b9afd) }

var fileDescriptor_33c57e4bae7b9afd = []byte{
	// 373 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x91, 0xcf, 0x8e, 0x9a, 0x50,
	0x14, 0xc6, 0xb9, 0x40, 0x45, 0x8f, 0xa8, 0xb7, 0xc7, 0xda, 0x92, 0xa6, 0x21, 0xc6, 0x95, 0x6d,
	0x52, 0x16, 0xf6, 0x09, 0x50, 0x6e, 0x2c, 0xa9, 0x7f, 0xc8, 0xe5, 0xba, 0xe8, 0xca, 0xd0, 0x48,
	0x4c, 0x63, 0x2d, 0x04, 0x4d, 0x13, 0xdf, 0xa2, 0xbb, 0x49, 0xe6, 0x89, 0x66, 0xe9, 0x72, 0x96,
	0x13, 0x7d, 0x91, 0x09, 0x17, 0xd4, 0x99, 0x1d, 0xdf, 0xf7, 0xfd, 0xb8, 0xe7, 0xcb, 0x39, 0xd0,
	0xd8, 0xc6, 0xbb, 0x5d, 0xb4, 0x8e, 0x9d, 0x34, 0x4b, 0xf6, 0x09, 0x1a, 0x69, 0x1c, 0x67, 0xdb,
	0x75, 0xd6, 0xbb, 0xd3, 0xc0, 0x98, 0x16, 0x11, 0x7e, 0x06, 0x7d, 0x7f, 0x48, 0x63, 0x8b, 0x74,
	0x49, 0xbf, 0x39, 0xe8, 0x38, 0x25, 0xe3, 0x94, 0xb9, 0x23, 0x0e, 0x69, 0xcc, 0x25, 0x82, 0x5f,
	0xc0, 0x48, 0xa3, 0xc3, 0x9f, 0x24, 0x5a, 0x59, 0x6a, 0x97, 0xf4, 0xeb, 0x03, 0x7a, 0xa5, 0x83,
	0xc2, 0xe7, 0x17, 0x00, 0x2d, 0x30, 0xfe, 0xc5, 0xd9, 0xee, 0x77, 0xf2, 0xd7, 0xd2, 0xba, 0xa4,
	0x5f, 0xe3, 0x17, 0xd9, 0xbb, 0x57, 0x41, 0xcf, 0x1f, 0xc5, 0x0e, 0xbc, 0x75, 0x83, 0x60, 0xf4,
	0xdd, 0xf5, 0x67, 0x4b, 0xce, 0xc6, 0x7e, 0x28, 0x18, 0xa7, 0x0a, 0xb6, 0xa1, 0x75, 0xb5, 0x17,
	0x81, 0xe7, 0x0a, 0x46, 0x09, 0x52, 0x30, 0xaf, 0xe6, 0x98, 0x09, 0xaa, 0xe2, 0x07, 0x68, 0xfb,
	0x33, 0xc1, 0x78, 0xe1, 0x4d, 0x99, 0x70, 0x65, 0xa0, 0x61, 0x0b, 0xea, 0x7c, 0x31, 0x61, 0x4b,
	0x8f, 0x05, 0x93, 0xf9, 0x4f, 0xaa, 0xa3, 0x09, 0x55, 0x7f, 0x28, 0x02, 0x19, 0xbf, 0xc1, 0x06,
	0xd4, 0xa4, 0x0a, 0xd9, 0xcc, 0xa3, 0x95, 0xbc, 0x84, 0x94, 0x9c, 0x8d, 0x98, 0x1f, 0x88, 0xc2,
	0x36, 0xf0, 0x1d, 0x50, 0x3e, 0x5f, 0x08, 0xc6, 0x97, 0x37, 0xb8, 0x8a, 0x9f, 0xc0, 0x7a, 0xe9,
	0xbe, 0xfa, 0xa7, 0x96, 0x0f, 0x76, 0x3d, 0x8f, 0xb3, 0x30, 0x94, 0xa3, 0x00, 0x3f, 0xc2, 0xfb,
	0x0b, 0x7e, 0x6b, 0x2a, 0xe1, 0x3a, 0x1a, 0xa0, 0xb9, 0xa3, 0x1f, 0xd4, 0xcc, 0xdb, 0x79, 0x6e,
	0x59, 0xbe, 0xd1, 0xfb, 0x0a, 0x46, 0xb9, 0x4a, 0x6c, 0x82, 0x9a, 0x6c, 0xe4, 0x59, 0xaa, 0x5c,
	0x4d, 0x36, 0x88, 0xa0, 0xaf, 0xa2, 0x7d, 0x24, 0x57, 0x6f, 0x72, 0xf9, 0x3d, 0xb4, 0x1e, 0x4e,
	0x36, 0x39, 0x9e, 0x6c, 0xf2, 0x74, 0xb2, 0xc9, 0xff, 0xb3, 0xad, 0x1c, 0xcf, 0xb6, 0xf2, 0x78,
	0xb6, 0x95, 0x5f, 0x15, 0x79, 0xf2, 0x6f, 0xcf, 0x01, 0x00, 0x00, 0xff, 0xff, 0xdc, 0x5b, 0x4a,
	0xda, 0x03, 0x02, 0x00, 0x00,
}

func (m *Message) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Message) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Message) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Version) > 0 {
		i -= len(m.Version)
		copy(dAtA[i:], m.Version)
		i = encodeVarintMessage(dAtA, i, uint64(len(m.Version)))
		i--
		dAtA[i] = 0x1a
	}
	if m.Payload != nil {
		{
			size, err := m.Payload.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintMessage(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.Type != 0 {
		i = encodeVarintMessage(dAtA, i, uint64(m.Type))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Payload) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Payload) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Payload) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Data) > 0 {
		i -= len(m.Data)
		copy(dAtA[i:], m.Data)
		i = encodeVarintMessage(dAtA, i, uint64(len(m.Data)))
		i--
		dAtA[i] = 0x12
	}
	if m.Ok {
		i--
		if m.Ok {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintMessage(dAtA []byte, offset int, v uint64) int {
	offset -= sovMessage(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Message) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Type != 0 {
		n += 1 + sovMessage(uint64(m.Type))
	}
	if m.Payload != nil {
		l = m.Payload.Size()
		n += 1 + l + sovMessage(uint64(l))
	}
	l = len(m.Version)
	if l > 0 {
		n += 1 + l + sovMessage(uint64(l))
	}
	return n
}

func (m *Payload) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Ok {
		n += 2
	}
	l = len(m.Data)
	if l > 0 {
		n += 1 + l + sovMessage(uint64(l))
	}
	return n
}

func sovMessage(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMessage(x uint64) (n int) {
	return sovMessage(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Message) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMessage
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Message: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Message: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= Message_Type(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Payload", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMessage
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMessage
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Payload == nil {
				m.Payload = &Payload{}
			}
			if err := m.Payload.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMessage
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMessage
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Version = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMessage(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMessage
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMessage
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Payload) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMessage
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Payload: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Payload: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ok", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Ok = bool(v != 0)
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthMessage
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthMessage
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data[:0], dAtA[iNdEx:postIndex]...)
			if m.Data == nil {
				m.Data = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMessage(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMessage
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMessage
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMessage(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMessage
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthMessage
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMessage
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMessage
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMessage        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMessage          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMessage = fmt.Errorf("proto: unexpected end of group")
)
