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

type central struct {
	t           transport.Transport
	sendCh      chan *transport.Frame
	rpcCh       chan *transport.Frame
	stopped     chan interface{}
	lock        sync.RWMutex
	streamArray [maxStreamNum]*Stream
}

func (c *central) Stream(id uint16) *Stream {
	return c.streamArray[id]
}

// CreateStream create an idle stream with io timeout duration
func (c *central) CreateStream(d time.Duration) (*Stream, error) {
	for i := int(reservedId + 1); i < maxStreamNum; i++ {
		if c.Stream(uint16(i)) == nil {
			c.lock.Lock()
			s, err := c.createStream(uint16(i), d)
			c.lock.Unlock()
			if err == nil {
				return s, nil
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

func (c *central) createStream(id uint16, timeout time.Duration) (*Stream, error) {
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
	for {
		select {
		case <-c.stopped:

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

func (c *central) destroyStream(s *Stream) {
	c.lock.Lock()
	c.streamArray[s.Id] = nil
	c.lock.Unlock()
}
