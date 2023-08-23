package codec

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

type ProtobufSerialization struct{}

func (p *ProtobufSerialization) Unmarshal(in []byte, body interface{}) error {
	if v, ok := body.(proto.Message); ok {
		return proto.Unmarshal(in, v)
	}
	return errors.New("proto unmarshal error, target should be proto.Message")
}

func (p *ProtobufSerialization) Marshal(body interface{}) (out []byte, err error) {
	if v, ok := body.(proto.Message); ok {
		return proto.Marshal(v)
	}
	return nil, errors.New("proto marshal error, target should be proto.Message")
}
