package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/transport"
)

const (
	maxStreamId uint16 = 65535
	reservedId  uint16 = 63
)

// NewCentral create new central for stream
func NewCentral(t transport.Transport) Central {
	c := &central{
		t:       t,
		sendCh:  make(chan *transport.Frame),
		rpcCh:   make(chan *transport.Frame),
		streams: map[uint16]*streamImpl{},
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go c.readPump()
	go c.writePump()
	return c
}

type Central interface {
	transport.Transport

	// Stream get existing Stream by id
	Stream(uint16) Stream

	// Open id Stream
	Open(id uint16, idle time.Duration) (Stream, error)

	// Listen create a new listening stream
	Listen(idle time.Duration) (Stream, error)
}

type central struct {
	t       transport.Transport
	sendCh  chan *transport.Frame
	rpcCh   chan *transport.Frame
	lock    sync.RWMutex
	streams map[uint16]*streamImpl
	ctx     context.Context
	cancel  context.CancelFunc
}

func (c *central) Stream(id uint16) Stream {
	return c.getStream(id)
}

func (c *central) Open(id uint16, idle time.Duration) (s Stream, err error) {
	if id <= reservedId || id >= maxStreamId {
		err = fmt.Errorf("invalid stream id %d", id)
		return
	}

	impl, err := c.createStream(id, idle, true)
	if err != nil {
		return
	}
	err = impl.open(idle)
	if err != nil {
		return
	}
	s = impl
	return
}

func (c *central) Listen(idle time.Duration) (Stream, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := reservedId + 1; i < maxStreamId; i++ {
		s, err := c.createStream(i, idle, false)
		if s != nil {
			return s, err
		}
	}
	return nil, fmt.Errorf("no more idle stream")
}

func (c *central) Read() (f *transport.Frame, err error) {
	select {
	case <-c.ctx.Done():
		err = c.ctx.Err()
	case f = <-c.rpcCh:
	}
	return
}

func (c *central) Write(f *transport.Frame) error {
	return c.t.Write(f)
}

func (c *central) Close() error {
	defer c.cancel()
	c.closeStreams()
	return c.t.Close()
}

func (c *central) createStream(id uint16, timeout time.Duration, sync bool) (*streamImpl, error) {
	if sync {
		c.lock.Lock()
		defer c.lock.Unlock()
	}

	if c.streams[id] != nil {
		return nil, fmt.Errorf("create stream error, %d exists already", id)
	}
	s := newStream(c.ctx, id, timeout, c.destroyStream)
	s.sendCh = c.sendCh
	c.streams[id] = s
	return s, nil
}

func (c *central) readPump() {
	for {
		f, err := c.t.Read()
		if err != nil {
			c.closeStreams()
			return
		}

		if f.Flag&transport.BinFlag != 0 {
			dst := f.Group
			if s := c.getStream(dst); s != nil {
				log.Printf("readpump stream %s", s)
				s.handleFrame(f)
			} else {
				// reset
				m := newFrame(nil)
				m.Opcode = transport.Close
				m.Group = f.Group
				c.sendCh <- m
			}
		} else if f.Flag&transport.RpcFlag != 0 {
			c.rpcCh <- f
		}
	}
}

func (c *central) writePump() {
	for {
		select {
		case <-c.ctx.Done():
			return

		case f := <-c.sendCh:
			err := c.t.Write(f)
			if err != nil {
				c.closeStreams()
				return
			}
		}
	}
}

func (c *central) getStream(id uint16) *streamImpl {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.streams[id]
}

func (c *central) destroyStream(s *streamImpl) {
	c.lock.Lock()
	delete(c.streams, s.id)
	c.lock.Unlock()
}

func (c *central) closeStreams() {
	c.lock.Lock()
	for _, s := range c.streams {
		s.close()
	}
	c.streams = nil
	c.lock.Unlock()
}
