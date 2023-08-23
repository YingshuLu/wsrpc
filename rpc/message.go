// Package rpc
package rpc

import (
	"encoding/binary"
	"errors"
)

const (
	RequestType uint16 = iota + 1
	ReplyType
	ErrorType
	CloseType
)

type Message struct {
	Type    uint16
	ID      uint32
	Service string
	Error   string
	Data    []byte
}

func (m *Message) Encode() ([]byte, error) {
	var buf = make([]byte, 0, 12)
	buf = appendUint16(buf, m.Type)
	buf = appendUint32(buf, m.ID)
	buf = appendUint16(buf, uint16(len(m.Service)))
	buf = append(buf, m.Service...)

	if m.Type == ErrorType || m.Type == CloseType {
		data := []byte(m.Error)
		buf = appendUint32(buf, uint32(len(data)))
		buf = append(buf, data...)
	} else {
		buf = appendUint32(buf, uint32(len(m.Data)))
		buf = append(buf, m.Data...)
	}
	return buf, nil
}

func (m *Message) Decode(data []byte) error {
	if len(data) < 12 {
		return errors.New("unmarshal error: on data less")
	}

	var index = 0
	m.Type = binary.BigEndian.Uint16(data[index:])
	index += 2

	m.ID = binary.BigEndian.Uint32(data[index:])
	index += 4

	var serviceLen = binary.BigEndian.Uint16(data[index:])
	index += 2

	m.Service = string(data[index : index+int(serviceLen)])
	index += int(serviceLen)

	var dataLen = binary.BigEndian.Uint32(data[index:])
	index += 4

	if m.Type == ErrorType || m.Type == CloseType {
		m.Error = string(data[index : index+int(dataLen)])
	} else {
		m.Data = data[index : index+int(dataLen)]
	}
	return nil
}

func appendUint16(buf []byte, v uint16) []byte {
	data16 := [2]byte{}
	binary.BigEndian.PutUint16(data16[:], v)
	return append(buf, data16[:]...)
}

func appendUint32(buf []byte, v uint32) []byte {
	data32 := [4]byte{}
	binary.BigEndian.PutUint32(data32[:], v)
	return append(buf, data32[:]...)
}
