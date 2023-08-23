// Package transport
package transport

import (
	"errors"
	"io"

	"github.com/gorilla/websocket"
)

func NewWebSocket(c *websocket.Conn) Transport {
	return New(&wsConn{Conn: c})
}

type wsConn struct {
	*websocket.Conn
	reader io.Reader
}

func (ws *wsConn) Read(buf []byte) (int, error) {
	var (
		typ int
		err error
	)
	if ws.reader == nil {
		typ, ws.reader, err = ws.NextReader()
		if err != nil {
			return 0, err
		}
		if typ == websocket.CloseMessage {
			return 0, errors.New("closed by peer")
		}
	}

	n, err := ws.reader.Read(buf)
	if err == io.EOF {
		ws.reader = nil
		err = nil
	}
	return n, err
}

func (ws *wsConn) Write(buf []byte) (int, error) {
	err := ws.Conn.WriteMessage(websocket.BinaryMessage, buf)
	return len(buf), err
}

func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}
