package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/transport"
)

const (
	maxStreamId uint16 = 65535
	reservedId  uint16 = 63
)

type Central interface {

	// Stream get existing Stream by id
	Stream(uint16) Stream

	// Open id Stream channel to remote
	Open(ctx context.Context, id uint16) (Stream, error)

	// Listen create listening stream
	Listen() (Stream, error)

	// Accept on listening stream
	Accept(ctx context.Context, id uint16) error

	// DispatchFrame dispatch frames to streams
	DispatchFrame(*transport.Frame)

	// Stop
	Stop()
}

// NewCentral create new central for stream
func NewCentral(ctx context.Context, sendFrame func(f *transport.Frame)) Central {
	c := &central{
		streams:   map[uint16]*streamImpl{},
		sendFrame: sendFrame,
		ctx:       ctx,
	}
	return c
}

type central struct {
	lock      sync.RWMutex
	streams   map[uint16]*streamImpl
	sendFrame func(*transport.Frame)
	ctx       context.Context
	cancel    context.CancelFunc
	closed    bool
}

func (c *central) Stream(id uint16) Stream {
	return c.getStream(id)
}

func (c *central) Open(ctx context.Context, id uint16) (s Stream, err error) {
	if id <= reservedId || id >= maxStreamId {
		err = fmt.Errorf("invalid stream id %d", id)
		return
	}

	impl, err := c.createStream(id, true)
	if err != nil {
		return
	}
	err = impl.open(ctx)
	if err != nil {
		return
	}
	s = impl
	return
}

func (c *central) Listen() (Stream, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := reservedId + 1; i < maxStreamId; i++ {
		s, err := c.createStream(i, false)
		if s != nil {
			return s, err
		}
	}
	return nil, fmt.Errorf("no more idle stream")
}

func (c *central) Accept(ctx context.Context, id uint16) error {
	s := c.getStream(id)
	if s == nil {
		return fmt.Errorf("not found stream %d, call Listen to create one", id)
	}

	return s.accept(ctx)
}

func (c *central) DispatchFrame(f *transport.Frame) {
	dst := f.Group
	if s := c.getStream(dst); s != nil {
		log.Printf("readpump stream %s", s)
		s.handleFrame(f)
	} else {
		f.Opcode = transport.Close
		c.writeFrame(f)
		log.Errorf("central: not found stream %v, send fin", dst)
	}
}

func (c *central) Stop() {
	if c.closed {
		return
	}
	c.closed = true
	c.clear()
}

func (c *central) writeFrame(f *transport.Frame) error {
	if c.closed {
		return errors.New("connection closed")
	}
	c.sendFrame(f)
	return nil
}

func (c *central) getStream(id uint16) *streamImpl {
	if c.closed {
		return nil
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.streams[id]
}

func (c *central) createStream(id uint16, sync bool) (*streamImpl, error) {
	if sync {
		c.lock.Lock()
		defer c.lock.Unlock()
	}

	if c.streams[id] != nil {
		return nil, fmt.Errorf("create stream error, %d exists already", id)
	}
	s := newStream(c.ctx, id, c.writeFrame, c.cleanStream)
	c.streams[id] = s
	return s, nil
}

func (c *central) cleanStream(s *streamImpl) {
	ss := c.streams
	if ss == nil {
		return
	}
	c.lock.Lock()
	delete(ss, s.id)
	c.lock.Unlock()
}

func (c *central) clear() {
	c.streams = nil
}
