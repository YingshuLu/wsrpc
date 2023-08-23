package stream

import (
	"context"
	"fmt"
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
	rpcFrame
)

const (
	openType = iota
	acceptType
)

const (
	defaultFragmentSize = 496
)

var null interface{}

type closeFunc func(stream *Stream)

func newStream(id uint16, timeout time.Duration, f closeFunc) *Stream {
	return &Stream{
		Id:    id,
		state: create,

		queue:  frame.NewQueue(),
		recvCh: make(chan *transport.Frame),

		fragmentSize: defaultFragmentSize,
		timeout:      timeout,
		peerClosed:   make(chan interface{}),
		closeFunc:    f,
	}
}

type Stream struct {
	Id            uint16
	Peer          uint16
	Type          int
	state         state
	index         uint16
	handshakeDone bool

	queue  frame.Queue
	recvCh chan *transport.Frame
	sendCh chan<- *transport.Frame

	fragmentSize int
	timeout      time.Duration
	peerClosed   chan interface{}
	closeFunc    func(stream *Stream)
}

func (s *Stream) String() string {
	return fmt.Sprintf("[direct %d <--> %d state %d]", s.Id, s.Peer, s.state)
}

func (s *Stream) Open(id uint16) error {
	if id <= reservedId || id >= maxStreamId {
		return fmt.Errorf("open stream error, illegal stream id: %d", id)
	}

	s.Peer = id
	s.Type = openType
	s.state = opening
	s.sendCh <- s.openFrame()

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	var err error
	select {
	case <-ctx.Done():
		err = fmt.Errorf("open stream %d failure, timeout", id)

	case f := <-s.recvCh:
		_, src := frameDstSrc(f)
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
			err = fmt.Errorf("open stream %d failure, local stream %d not listening", id, s.Id)
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

func (s *Stream) Accept() error {
	s.Type = acceptType
	s.state = accepting
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	var err error
	select {
	case <-ctx.Done():
		err = fmt.Errorf("accept stream %d failure, timeout", s.Id)
		break
	case f := <-s.recvCh:
		dst, src := frameDstSrc(f)
		if dst != s.Id {
			s.fin()
			err = fmt.Errorf("accept stream %d failure, dst id not matched", s.Id)
			s.state = closed
			break
		}
		switch frameType(f) {
		case openFrame:
			s.state = streaming
			s.Peer = src
			s.sendCh <- s.acceptFrame()
			s.handshakeDone = true
		case acceptFrame:
			s.fin()
			s.state = closed
			err = fmt.Errorf("accept stream %d failure, local stream %d not opening", src, s.Id)
		case streamFrame:
			s.queue.Push(f)
			s.state = streaming
		case finishedFrame:
			s.fin()
			s.state = closed
			err = fmt.Errorf("accept stream %d failure, finished frame", s.Id)
		}
	}

	return err
}

func (s *Stream) handleFrame(f *transport.Frame) {
	log.Printf("handle stream %s, frame type %d", s, frameType(f))
	if s.state != create && s.state != destroy && s.state != streaming && s.state != timeWait && s.state != closeWait {
		s.recvCh <- f
		return
	}

	switch frameType(f) {
	case openFrame:
		if s.Type == acceptType && !s.handshakeDone {
			s.handshakeDone = true
		} else {
			s.fin()
			s.state = closed
		}
	case acceptFrame:
		// ignore
		if s.Type == openType && !s.handshakeDone {
			s.handshakeDone = true
		} else {
			s.fin()
			s.state = closed
		}
	case streamFrame:
		s.recvCh <- f

	case finishedFrame:
		log.Printf("[fin] stream %s read fin frame", s)
		if s.state == timeWait {
			s.peerClosed <- null
		} else {
			s.state = closeWait
		}
	}
}

func (s *Stream) Read(p []byte) (int, error) {
	if s.state != streaming && s.state != timeWait && s.state != closeWait {
		return 0, fmt.Errorf("read stream %s error", s)
	}

	total := len(p)
	if total == 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	for s.queue.Peek() == nil {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("read stream %s error, timeout", s)
		case f := <-s.recvCh:
			s.queue.Push(f)
		}
	}

	// have available data to read
	left := total
	for f := s.queue.Peek(); f != nil && left > 0; f = s.queue.Peek() {
		n := copy(p[total-left:], f.Payload)
		log.Printf("read %s frame payload size: %d left: %d copy: %d", s, len(f.Payload), left, n)
		f.Payload = f.Payload[n:]
		left -= n

		if len(f.Payload) == 0 {
			log.Println("read queue pop")
			s.queue.Pop()
		}
	}

	return total - left, nil
}

func (s *Stream) Write(p []byte) (int, error) {
	if s.state != streaming && s.state != closeWait && s.state != create {
		return 0, fmt.Errorf("write closed stream %s with error", s)
	}
	if len(p) == 0 {
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
		f := s.streamFrame(s.Peer, s.nextIndex(), p[index:index+dataLen])
		index += dataLen
		p = p[index:]
		s.sendCh <- f
	}
	return 0, nil
}

func (s *Stream) Close() error {
	defer s.destroy()

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

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return fmt.Errorf("close stream %s timeout", s)
	case <-s.peerClosed:
		s.state = closed
	}
	return nil
}

func (s *Stream) destroy() {
	if s.closeFunc != nil && s.state != destroy {
		s.closeFunc(s)
		s.state = destroy
	}
}

func (s *Stream) fin() {
	if s.Peer > 0 {
		f := s.finishedFrame(s.Peer, s.nextIndex())
		s.sendCh <- f
	}
}

func (s *Stream) openFrame() *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Open
	setFrameDstSrc(f, s.Peer, s.Id)
	f.Flag |= transport.NextFlag
	return f
}

func (s *Stream) acceptFrame() *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Open
	setFrameDstSrc(f, s.Peer, s.Id)
	f.Flag |= transport.NextFlag
	f.Flag |= transport.AckFlag
	return f
}

func (s *Stream) streamFrame(id, index uint16, payload []byte) *transport.Frame {
	f := newFrame(payload)
	f.Opcode = transport.Stream
	setFrameDstSrc(f, s.Peer, s.Id)
	f.Group = id
	f.Index = index
	f.Flag |= transport.NextFlag
	return f
}

func (s *Stream) finishedFrame(id, index uint16) *transport.Frame {
	f := newFrame(nil)
	f.Opcode = transport.Close
	setFrameDstSrc(f, s.Peer, s.Id)
	f.Group = id
	f.Index = index
	return f
}

func (s *Stream) nextIndex() uint16 {
	s.index++
	return s.index
}

func newFrame(payload []byte) *transport.Frame {
	f := &transport.Frame{
		Magic:   transport.MagicCode,
		Length:  uint32(len(payload)),
		Payload: payload,
	}
	f.Flag |= transport.BinFlag

	for _, c := range payload {
		f.Reserved ^= c
	}
	return f
}

// from frame receiver side
func setFrameDstSrc(f *transport.Frame, dst, src uint16) {
	f.CheckSum = uint32(dst)<<16 | uint32(src)
}

// from frame receiver side
func frameDstSrc(f *transport.Frame) (uint16, uint16) {
	src := uint16(f.CheckSum & (uint32(1<<17) - 1))
	dst := uint16(f.CheckSum >> 16)
	return dst, src
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
