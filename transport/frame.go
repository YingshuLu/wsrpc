// Package transport
package transport

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

const (
	MagicCode         byte = 0x6f
	HeaderFixedLength      = 16
)

const (
	NextFlag byte = 1 << 3
	BinFlag  byte = 1 << 2
	RpcFlag  byte = 1 << 1
	AckFlag  byte = 1
)

const (
	None byte = iota
	Stream
	Open
	Close
)

func NewFrame(data []byte) *Frame {
	return &Frame{
		Magic:    MagicCode,
		CheckSum: crc32.ChecksumIEEE(data),
		Length:   uint32(len(data)),
		Payload:  data,
	}
}

func NewRpcFrame(data []byte) *Frame {
	f := NewFrame(data)
	f.Flag |= RpcFlag
	return f
}

type Frame struct {
	Magic    byte
	Flag     byte
	Opcode   byte
	Reserved byte
	CheckSum uint32
	Group    uint16
	Index    uint16
	Length   uint32
	Payload  []byte
}

func (f *Frame) SetNext() {
	f.Flag |= NextFlag
}

func (f *Frame) HasNext() bool {
	return (f.Flag & NextFlag) != 0
}

func (f *Frame) Encode() []byte {
	var buf = make([]byte, HeaderFixedLength+len(f.Payload))
	buf[0] = f.Magic
	buf[1] = f.Flag
	buf[2] = f.Opcode
	buf[3] = f.Reserved
	binary.BigEndian.PutUint32(buf[4:], f.CheckSum)
	binary.BigEndian.PutUint16(buf[8:], f.Group)
	binary.BigEndian.PutUint16(buf[10:], f.Index)
	binary.BigEndian.PutUint32(buf[12:], f.Length)
	copy(buf[16:], f.Payload)
	return buf
}

func (f *Frame) DecodeHeader(data []byte) error {
	if data[0] != MagicCode {
		return errors.New("decode error: on magic code unmatched")
	}
	f.Magic = data[0]
	f.Flag = data[1]
	f.Opcode = data[2]
	f.Reserved = data[3]
	f.CheckSum = binary.BigEndian.Uint32(data[4:])
	f.Group = binary.BigEndian.Uint16(data[8:])
	f.Index = binary.BigEndian.Uint16(data[10:])
	f.Length = binary.BigEndian.Uint32(data[12:])
	return nil
}

func (f *Frame) Decode(data []byte) error {
	if len(data) < HeaderFixedLength {
		return errors.New("decode error: on data length less")
	}

	err := f.DecodeHeader(data)
	if err != nil {
		return err
	}
	f.Payload = data[HeaderFixedLength : HeaderFixedLength+f.Length]
	return nil
}

func newFrameReader(r io.Reader) *frameReader {
	return &frameReader{
		r: bufio.NewReader(r),
	}
}

func newFrameWriter(w io.Writer) *frameWriter {
	return &frameWriter{
		w: w,
	}
}

type frameReader struct {
	r         *bufio.Reader
	headerBuf [HeaderFixedLength]byte
}

func (f *frameReader) Read() (*Frame, error) {
	_, err := io.ReadFull(f.r, f.headerBuf[:])
	if err != nil {
		return nil, err
	}

	m := &Frame{}
	err = m.DecodeHeader(f.headerBuf[:])
	if err != nil {
		return nil, err
	}

	m.Payload = make([]byte, int(m.Length))
	_, err = io.ReadFull(f.r, m.Payload)
	return m, err
}

type frameWriter struct {
	w io.Writer
}

func (f *frameWriter) Write(m *Frame) error {
	data := m.Encode()
	_, err := f.w.Write(data)
	return err
}
