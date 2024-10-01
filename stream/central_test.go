package stream

import (
	"context"
	"testing"
	"time"

	"github.com/yingshulu/wsrpc/transport"
)

func newTransport() *mockTransport {
	return &mockTransport{
		readChan:  make(chan *transport.Frame),
		writeChan: make(chan *transport.Frame),
	}
}

type mockTransport struct {
	readChan  chan *transport.Frame
	writeChan chan *transport.Frame
}

func (m *mockTransport) Read() (*transport.Frame, error) {
	f := <-m.readChan
	return f, nil
}

func (m *mockTransport) Write(f *transport.Frame) error {
	m.writeChan <- f
	return nil
}

func (m *mockTransport) Close() error {
	close(m.readChan)
	close(m.writeChan)
	return nil
}

func TestOpenAccept(t *testing.T) {
	tc := newTransport()
	ts := newTransport()
	tc.readChan, tc.writeChan = ts.writeChan, ts.readChan

	cc := &central{
		t:       tc,
		sendCh:  make(chan *transport.Frame),
		streams: map[uint16]*streamImpl{},
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())

	go cc.readPump()
	go cc.writePump()

	cs := &central{
		t:       ts,
		sendCh:  make(chan *transport.Frame),
		streams: map[uint16]*streamImpl{},
	}
	cs.ctx, cs.cancel = context.WithCancel(context.Background())

	go cs.readPump()
	go cs.writePump()

	ss, err := cs.Listen()
	if err != nil {
		t.Log("[cc] open stream error: ", err)
		return
	}

	id := ss.Id()
	time.Sleep(time.Second)
	go func() {
		s, err := cc.Open(cc.ctx, id)
		if err != nil {
			t.Log("[cc] open stream error: ", err)
			return
		}
		t.Logf("[cc] open stream %s success", s)

		s.(*streamImpl).sendCh <- s.(*streamImpl).streamFrame(s.(*streamImpl).id, 3, []byte("3. good message."))
		_, err = s.Write(cc.ctx, []byte("1. hello world, "))
		if err != nil {
			t.Log("[cc] write stream error: ", err)
			return
		}
		t.Log("[cc] write stream success")
		s.(*streamImpl).sendCh <- s.(*streamImpl).streamFrame(s.(*streamImpl).id, 2, []byte("2. this is second, "))

		b := make([]byte, 128)
		n, err := s.Read(cc.ctx, b)
		if err != nil {
			t.Log("[cc] read stream error: ", err)
			return
		}
		t.Log("[cc] read stream: ", string(b[0:n]))

		err = s.Close()
		if err != nil {
			t.Log("[cc] fin stream error: ", err)
			return
		}
		t.Logf("[cc] fin stream %s success", s)
	}()

	err = cs.Accept(cs.ctx, ss.Id())
	if err != nil {
		t.Logf("[cs] accept stream error %v", err)
		return
	}
	t.Log("[cs] accept stream success")
	b := make([]byte, 128)
	for i := 0; i < 51; {
		n, err := ss.Read(cs.ctx, b[i:])
		if err != nil {
			t.Log("[cs] read stream error: ", err)
			return
		}
		i += n
	}
	t.Log("[cs] read stream success, ", string(b[0:51]))

	_, err = ss.Write(cc.ctx, []byte("this from cs message"))
	if err != nil {
		t.Log("[cs] write stream error: ", err)
		return
	}
	t.Log("[cs] write stream success")
	time.Sleep(2 * time.Second)
	t.Log("[cs] stream state: ", ss)
	err = ss.Close()
	if err != nil {
		t.Log("[cs] closeWait stream error: ", err)
		return
	}
	t.Logf("[cs] close stream %s success", ss)
	time.Sleep(3 * time.Second)
}
