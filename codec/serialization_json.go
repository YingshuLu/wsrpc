package codec

import (
	jsoniter "github.com/json-iterator/go"
)

var j = jsoniter.ConfigCompatibleWithStandardLibrary

// JSONSerialization 序列化json包体 标准包使用了反射来实现，性能极低，使用json-iterator解码能提升5倍性能，编码也比标准包性能好
type JSONSerialization struct{}

// Unmarshal 反序列json
func (s *JSONSerialization) Unmarshal(in []byte, body interface{}) error {
	return j.Unmarshal(in, body)
}

// Marshal 序列化json
func (s *JSONSerialization) Marshal(body interface{}) ([]byte, error) {
	return j.Marshal(body)
}
