// Package codec
package codec

import "errors"

// Serializer body序列化接口
type Serializer interface {
	Unmarshal(in []byte, body interface{}) error
	Marshal(body interface{}) (out []byte, err error)
}

var serializers = map[string]Serializer{
	"json":     &JSONSerialization{},
	"protobuf": &ProtobufSerialization{},
}

// RegisterSerializer 注册序列化具体实现Serializer，由第三方包的init函数调用
func RegisterSerializer(serializationType string, s Serializer) {
	serializers[serializationType] = s
}

// GetSerializer 通过serialization type获取Serializer
func GetSerializer(serializationType string) Serializer {
	return serializers[serializationType]
}

// Unmarshal 解析body，内部通过不同serialization type，调用不同的序列化方式，默认protobuf
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

// Marshal 打包body，内部通过不同serialization type，调用不同的序列化方式, 默认protobuf
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
