// Package codec
package codec

import "errors"

type Serializer interface {
	Unmarshal(in []byte, body interface{}) error
	Marshal(body interface{}) (out []byte, err error)
}

var serializers = map[string]Serializer{
	"json":     &JSONSerialization{},
	"protobuf": &ProtobufSerialization{},
}

func RegisterSerializer(serializationType string, s Serializer) {
	serializers[serializationType] = s
}

func GetSerializer(serializationType string) Serializer {
	return serializers[serializationType]
}

func Unmarshal(serializationType string, in []byte, body interface{}) error {
	if body == nil {
		return nil
	}
	if len(in) == 0 {
		return nil
	}

	s := GetSerializer(serializationType)
	if s == nil {
		return errors.New("serializer not registered")
	}

	return s.Unmarshal(in, body)
}

func Marshal(serializationType string, body interface{}) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	s := GetSerializer(serializationType)
	if s == nil {
		return nil, errors.New("serializer not registered")
	}
	return s.Marshal(body)
}
