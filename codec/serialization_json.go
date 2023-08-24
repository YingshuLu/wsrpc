package codec

import (
	jsoniter "github.com/json-iterator/go"
)

var j = jsoniter.ConfigCompatibleWithStandardLibrary

type JSONSerialization struct{}

func (s *JSONSerialization) Unmarshal(in []byte, body interface{}) error {
	return j.Unmarshal(in, body)
}

func (s *JSONSerialization) Marshal(body interface{}) ([]byte, error) {
	return j.Marshal(body)
}
