package stream

import (
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
	timeout := 10 * time.Second
	tc := newTransport()
	ts := newTransport()
	tc.readChan, tc.writeChan = ts.writeChan, ts.readChan

	cc := &central{
		t:      tc,
		sendCh: make(chan *transport.Frame),
	}

	go cc.readPump()
	go cc.writePump()

	cs := &central{
		t:      ts,
		sendCh: make(chan *transport.Frame),
	}

	go cs.readPump()
	go cs.writePump()

	ss, err := cs.CreateStream(timeout)
	if err != nil {
		t.Log("[cc] open stream error: ", err)
		return
	}

	id := ss.Id()
	time.Sleep(time.Second)
	go func() {
		s, err := cc.CreateStream(timeout)
		if err != nil {
			t.Log("[cc] open stream error: ", err)
			return
		}
		err = s.Open(id)
		if err != nil {
			t.Log("[cc] open stream error: ", err)
			return
		}
		t.Logf("[cc] open stream %s success", s)

		s.(*streamImpl).sendCh <- s.(*streamImpl).streamFrame(s.(*streamImpl).peer, 3, []byte("3. good message."))
		_, err = s.Write([]byte("1. hello world, "))
		if err != nil {
			t.Log("[cc] write stream error: ", err)
			return
		}
		t.Log("[cc] write stream success")
		s.(*streamImpl).sendCh <- s.(*streamImpl).streamFrame(s.(*streamImpl).peer, 2, []byte("2. this is second, "))

		b := make([]byte, 128)
		n, err := s.Read(b)
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

	err = ss.Accept()
	if err != nil {
		t.Logf("[cs] accept stream error %v", err)
		return
	}
	t.Log("[cs] accept stream success")
	b := make([]byte, 128)
	for i := 0; i < 51; {
		n, err := ss.Read(b[i:])
		if err != nil {
			t.Log("[cs] read stream error: ", err)
			return
		}
		i += n
	}
	t.Log("[cs] read stream success, ", string(b[0:51]))

	_, err = ss.Write([]byte("this from cs message"))
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
