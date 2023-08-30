package stream

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/transport"
)

const (
	maxStreamNum        = 1 << 16
	maxStreamId  uint16 = maxStreamNum - 1
	reservedId   uint16 = 255
)

// NewCentral create new central for stream
func NewCentral(t transport.Transport) Central {
	c := &central{
		t:       t,
		sendCh:  make(chan *transport.Frame),
		rpcCh:   make(chan *transport.Frame),
		stopped: make(chan interface{}),
	}
	go c.readPump()
	go c.writePump()
	return c
}

type Central interface {
	// Stream get existing Stream by id
	Stream(uint16) Stream

	// CreateStream create new Stream
	CreateStream(time.Duration) (Stream, error)

	// Read a RPC frame
	Read() (*transport.Frame, error)

	// Write a RPC frame
	Write(f *transport.Frame) error

	// Close the connection
	Close() error
}

type central struct {
	t           transport.Transport
	sendCh      chan *transport.Frame
	rpcCh       chan *transport.Frame
	stopped     chan interface{}
	lock        sync.RWMutex
	streamArray [maxStreamNum]*streamImpl
}

func (c *central) Stream(id uint16) Stream {
	s := c.streamArray[id]
	if s == nil {
		return nil
	}
	return s
}

// CreateStream create an idle stream with io timeout duration
func (c *central) CreateStream(d time.Duration) (s Stream, err error) {
	for i := int(reservedId + 1); i < maxStreamNum; i++ {
		if c.Stream(uint16(i)) == nil {
			c.lock.Lock()
			if c.Stream(uint16(i)) == nil {
				s, err = c.createStream(uint16(i), d)
			}
			c.lock.Unlock()
			if s != nil {
				return
			}
		}
	}
	return nil, fmt.Errorf("no more idle stream")
}

func (c *central) Read() (*transport.Frame, error) {
	f := <-c.rpcCh
	if f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("read error")
}

func (c *central) Write(f *transport.Frame) error {
	return c.t.Write(f)
}

func (c *central) Close() error {
	for _, s := range c.streamArray {
		if s != nil {
			s.Close()
		}
	}

	c.stopped <- null
	return c.t.Close()
}

func (c *central) createStream(id uint16, timeout time.Duration) (Stream, error) {
	if c.streamArray[id] != nil {
		return nil, fmt.Errorf("create stream error, %d exists already", id)
	}
	s := newStream(id, timeout, c.destroyStream)
	s.sendCh = c.sendCh
	c.streamArray[id] = s
	return s, nil
}

func (c *central) readPump() {
	for {
		f, err := c.t.Read()
		if err != nil {
			return
		}

		if f.Flag&transport.BinFlag != 0 {
			dst, src := frameDstSrc(f)
			if s := c.streamArray[dst]; s != nil {
				log.Printf("readpump stream %s", s)
				s.handleFrame(f)
			} else {
				// reset
				m := newFrame(nil)
				m.Opcode = transport.Close
				setFrameDstSrc(m, src, dst)
				m.Group = f.Group
				c.sendCh <- m
			}
		} else if f.Flag&transport.RpcFlag != 0 {
			c.rpcCh <- f
		}
	}
}

func (c *central) writePump() {
	centralStopped := false
	for !centralStopped {
		select {
		case <-c.stopped:
			centralStopped = true

		case f := <-c.sendCh:
			err := c.t.Write(f)
			if err != nil {
				dst, _ := frameDstSrc(f)
				if s := c.streamArray[dst]; s != nil {
					s.Close()
				}
			}
		}
	}
}

func (c *central) destroyStream(s *streamImpl) {
	c.lock.Lock()
	c.streamArray[s.id] = nil
	c.lock.Unlock()
}
