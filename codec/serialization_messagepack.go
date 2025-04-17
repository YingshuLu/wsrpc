package codec

import (
	"github.com/vmihailenco/msgpack/v5"
)

type MessagePackSerialization struct{}

func (p *MessagePackSerialization) Unmarshal(in []byte, body interface{}) error {
	return msgpack.Unmarshal(in, body)
}

func (p *MessagePackSerialization) Marshal(body interface{}) ([]byte, error) {
	return msgpack.Marshal(body)
}
