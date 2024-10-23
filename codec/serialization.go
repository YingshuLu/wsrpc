// Package codec
package codec

import "errors"

const (
	None Type = iota
	Protobuf
	Json
	Unknown
)

var stringTypes = []string{"none", "protobuf", "json"}

type Type uint8

func (t Type) String() string {
	if !t.Legal() {
		return "Unknown"
	}
	return stringTypes[t]
}

func (t Type) Legal() bool {
	return t > None && t < Unknown
}

func TypeOf(serializationType string) Type {
	switch serializationType {
	case "protobuf":
		return Protobuf
	case "json":
		return Json
	default:
		return None
	}
}

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
