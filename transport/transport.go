// Package transport
package transport

import "io"

func New(c io.ReadWriteCloser) Transport {
	return &transport{
		frameReader: newFrameReader(c),
		frameWriter: newFrameWriter(c),
		Closer:      c,
	}
}

type Transport interface {
	Read() (*Frame, error)
	Write(*Frame) error
	Close() error
}

type transport struct {
	*frameReader
	*frameWriter
	io.Closer
}
