package stream

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/stream/frame"
	"github.com/yingshulu/wsrpc/transport"
)

type state int

const (
	create state = iota
	opening
	accepting
	streaming
	finSent
	timeWait
	closeWait
	closed
	destroy
)

const (
	illegalFrame = iota
	openFrame
	acceptFrame
	streamFrame
	finishedFrame
)

const (
	openType = iota
	acceptType
)

const (
	defaultFragmentSize = 496
)

var null interface{}

type closeFunc func(stream *streamImpl)

type Stream interface {
	// String output stream details
	String() string

	// Id local id
	Id() uint16

	// Read data
	Read(ctx context.Context, buf []byte) (int, error)

	// Write data
	Write(ctx context.Context, data []byte) (int, error)

	// Close the stream
	Close() error
}

func newStream(ctx context.Context, id uint16, sendFrame func(*transport.Frame), f closeFunc) *streamImpl {
	s := &streamImpl{
		id:    id,
		state: create,

		queue: frame.NewQueue(),
		revCh: make(chan *transport.Frame),

		fragmentSize: defaultFragmentSize,
		peerClosed:   make(chan interface{}),
		closeFunc:    f,
		sendFrame:    sendFrame,
	}
	s.parentCtx, s.cancel = context.WithCancel(ctx)
	return s
}

type streamImpl struct {
	id            uint16
	typ           int
	state         state
	index         atomic.Uint32
	handshakeDone bool
	queue         frame.Queue
	revCh         chan *transport.Frame
	sendFrame     func(*transport.Frame)

	fragmentSize int
	peerClosed   chan interface{}
	parentCtx    context.Context
	cancel       context.CancelFunc
	closeFunc    func(stream *streamImpl)

	leftStreamFrames atomic.Int32
}

func (s *streamImpl) String() string {
	direct := "----> remote"
	if s.typ == acceptType {
		direct = "<---- remote"
	}
	return fmt.Sprintf("[stream %d %s state %d]", s.id, direct, s.state)
}

func (s *streamImpl) Id() uint16 {
	return s.id
}

func (s *streamImpl) open(ctx context.Context) error {
	id := s.id
	s.typ = openType
	s.state = opening
	s.sendFrame(s.openFrame())

	var err error
	select {
	case <-s.parentCtx.Done():
		err = fmt.Errorf("open stream %d failure, parent - %v", id, ctx.Err())

	case <-ctx.Done():
		err = fmt.Errorf("open stream %d failure, %v", id, ctx.Err())

	case f := <-s.revCh:
		src := f.Group
		if src != id {
			s.fin()
			s.state = closed
			err = fmt.Errorf("open stream %d failure, src id not matched", id)
			break
		}

		switch frameType(f) {
		case openFrame:
			s.fin()
			s.state = closed
			err = fmt.Errorf("open stream %d failure, local stream %d not listening", id, s.id)
		case acceptFrame:
			s.state = streaming
			s.handshakeDone = true
		case streamFrame:
			s.queue.Push(f)
			s.state = streaming
		case finishedFrame:
			s.peerClosed <- null
			s.state = closed
			err = fmt.Errorf("open stream %d failure, finished frame", id)
		}
	}

	if err != nil {
		s.state = closed
	}
	return err
}

func (s *streamImpl) accept(ctx context.Context) error {
	s.typ = acceptType
	s.state = accepting

	var err error
	select {
	case <-s.parentCtx.Done():
		err = fmt.Errorf("accept stream %d failure, parent - %v", s.id, ctx.Err())

	case <-ctx.Done():
		err = fmt.Errorf("accept stream %d failure, %v", s.id, ctx.Err())

	case f := <-s.revCh:
		dst := f.Group
		if dst != s.id {
			s.fin()
			err = fmt.Errorf("accept stream %d failure, dst id not matched", s.id)
			s.state = closed
			break
		}
		switch frameType(f) {
		case openFrame:
			s.state = streaming
			s.sendFrame(s.acceptFrame())
			s.handshakeDone = true
		case acceptFrame:
			s.fin()
			s.state = closed
			err = fmt.Errorf("accept stream %d failure, local stream %d not opening", f.Group, s.id)
		case streamFrame:
			s.queue.Push(f)
			s.state = streaming
		case finishedFrame:
			s.fin()
			s.state = closed
			err = fmt.Errorf("accept stream %d failure, finished frame", s.id)
		}
	}

	return err
}

func (s *streamImpl) handleFrame(f *transport.Frame) {
	log.Debugf("handle stream %s, frame type %d", s, frameType(f))
	if s.state != create && s.state != destroy && s.state != streaming && s.state != timeWait && s.state != closeWait {
		s.revCh <- f
		return
	}

	switch frameType(f) {
	case openFrame:
		if s.typ == acceptType && !s.handshakeDone {
			s.handshakeDone = true
		} else {
			s.fin()
			s.state = closed
		}
	case acceptFrame:
		// ignore
		if s.typ == openType && !s.handshakeDone {
			s.handshakeDone = true
		} else {
			s.fin()
			s.state = closed
		}
	case streamFrame:
		s.leftStreamFrames.Add(1)
		s.revCh <- f

	case finishedFrame:
		log.Debugf("[fin] stream %s read fin frame", s)
		if s.state == timeWait {
			s.peerClosed <- null
		} else {
			s.state = closeWait
			// exit Read
			if s.leftStreamFrames.Load() == 0 {
				s.cancel()
			}
		}
	}
}

func (s *streamImpl) Read(ctx context.Context, p []byte) (int, error) {
	if s.state == closeWait || s.state == closed {
		return 0, io.EOF
	}

	if s.state != streaming && s.state != timeWait {
		return 0, fmt.Errorf("read stream %s error", s)
	}

	total := len(p)
	if total == 0 {
		return 0, nil
	}

	// reorder frames
	for s.queue.Peek() == nil {
		select {
		case <-s.parentCtx.Done():
			if s.state == closeWait || s.state == closed {
				return 0, io.EOF
			}
			return 0, fmt.Errorf("read stream %s error, parent - %v", s, ctx.Err())

		case <-ctx.Done():
			if s.state == closeWait || s.state == closed {
				return 0, io.EOF
			}
			return 0, fmt.Errorf("read stream %s error, %v", s, ctx.Err())

		case f := <-s.revCh:
			s.leftStreamFrames.Add(-1)
			s.queue.Push(f)
		}
	}

	// have available data to read
	left := total
	for f := s.queue.Peek(); f != nil && left > 0; f = s.queue.Peek() {
		n := copy(p[total-left:], f.Payload)
		log.Debugf("read %s frame payload size: %d left: %d copy: %d", s, len(f.Payload), left, n)
		f.Payload = f.Payload[n:]
		left -= n

		if len(f.Payload) == 0 {
			s.queue.Pop()
		}
	}

	return total - left, nil
}

func (s *streamImpl) Write(_ context.Context, p []byte) (int, error) {
	if s.state != streaming && s.state != closeWait && s.state != create {
		return 0, fmt.Errorf("write closed stream %s with error", s)
	}

	total := len(p)
	if total == 0 {
		return 0, nil
	}

	index := 0
	dataLen := 0
	for len(p) > 0 {
		if len(p) >= s.fragmentSize {
			dataLen = s.fragmentSize
		} else {
			dataLen = len(p)
		}
		f := s.streamFrame(s.id, s.nextIndex(), p[index:index+dataLen])
		index += dataLen
		p = p[index:]
		s.sendFrame(f)
	}
	return total, nil
}

func (s *streamImpl) Close() error {
	defer s.destroy()
	defer s.cancel()
	return s.close()
}

func (s *streamImpl) close() error {
	switch s.state {
	case closed, finSent, timeWait:
		return nil
	}

	switch s.state {
	case closeWait:
		s.fin()
		s.state = closed
		return nil
	case streaming:
		s.fin()
		s.state = timeWait
	}

	timeout := time.After(5 * time.Second)
	select {
	case <-timeout:
		s.state = closed
		return fmt.Errorf("close stream %s timeout", s)

	case <-s.peerClosed:
		s.state = closed
	}
	return nil
}

func (s *streamImpl) destroy() {
	if s.closeFunc != nil && s.state != destroy {
		s.closeFunc(s)
		s.state = destroy
	}
}

func (s *streamImpl) fin() {
	f := s.finishedFrame(s.id, s.nextIndex())
	s.sendFrame(f)
}

func (s *streamImpl) openFrame() *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Open
	f.Group = s.id
	f.Flag |= transport.NextFlag
	return f
}

func (s *streamImpl) acceptFrame() *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Open
	f.Group = s.id
	f.Flag |= transport.NextFlag
	f.Flag |= transport.AckFlag
	return f
}

func (s *streamImpl) streamFrame(id, index uint16, payload []byte) *transport.Frame {
	f := newFrame(payload)
	f.Opcode = transport.Stream
	f.Group = id
	f.Index = index
	f.Flag |= transport.NextFlag
	return f
}

func (s *streamImpl) finishedFrame(id, index uint16) *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Close
	f.Group = id
	f.Index = index
	return f
}

func (s *streamImpl) nextIndex() uint16 {
	if s.index.CompareAndSwap(65535, 1) {
		return 1
	}
	return uint16(s.index.Add(1))
}

func newFrame(payload []byte) *transport.Frame {
	f := &transport.Frame{
		Magic:   transport.MagicCode,
		Length:  uint32(len(payload)),
		Payload: payload,
	}
	f.Flag |= transport.BinFlag
	return f
}

func frameType(f *transport.Frame) int {
	if f.Flag&transport.BinFlag == 0 {
		return illegalFrame
	}

	switch f.Opcode {
	case transport.Open:
		if f.Flag&transport.AckFlag != 0 {
			return acceptFrame
		}
		return openFrame
	case transport.Stream:
		return streamFrame
	case transport.Close:
		return finishedFrame
	}

	return illegalFrame
}
