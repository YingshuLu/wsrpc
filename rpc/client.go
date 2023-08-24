// Package rpc
package rpc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc/service/keepalive"
	"github.com/yingshulu/wsrpc/transport"
)

func NewClient(host string, options ...Option) *Client {
	c := &Client{
		Host:          host,
		ServiceHolder: newServiceHolder(),
		stopped:       make(chan interface{}),
		options:       defaultOptions(),
	}
	c.options.Apply(options)
	go c.scheduleKeepalive()
	return c
}

type Client struct {
	ServiceHolder
	Host    string
	addr    string
	stopped chan interface{}
	options *Options
}

func (c *Client) Connect(a *Addr) error {
	if ss := strings.Split(a.Host, "://"); len(ss) > 1 && len(a.Schema) == 0 {
		a.Schema = ss[0]
	}

	switch a.Schema {
	case "tcp", "udp":
		return c.dialConnect(a, a.Schema)
	case "ws", "wss":
		return c.wsConnect(a)
	}
	return fmt.Errorf("not support connection: %v", a)
}

func (c *Client) wsConnect(a *Addr) error {
	header := http.Header{}
	header.Add(hostIdKey, c.Host)
	wsc, resp, err := websocket.DefaultDialer.Dial(a.Host, header)
	if err != nil {
		return err
	}

	id := resp.Header.Get(connIdKey)
	peer := resp.Header.Get(hostIdKey)
	a.Name = peer
	c.onConnected(transport.NewWebSocket(wsc), peer, id, a)
	return nil
}

func (c *Client) dialConnect(a *Addr, typ string) error {
	peer := a.Name
	id := uuid.NewString()
	tc, err := net.Dial(typ, fmt.Sprintf("%s:%d", a.Host, a.Port))
	if err != nil {
		return err
	}
	c.onConnected(transport.New(tc), peer, id, a)
	return nil
}

func (c *Client) Close() {
	c.stopped <- null
}

func (c *Client) onConnected(t transport.Transport, peer, id string, a *Addr) {
	conn := newConn(t, c, peer, id, true)
	conn.addr = a
	c.AddConn(conn)
}

func (c *Client) scheduleKeepalive() {
	methodName := keepalive.ServiceName + ".keepalive"
	ctx := context.Background()
	for {
		select {

		case <-time.After(c.options.KeepaliveTimeout):
			conns := c.GetConns()
			for _, conn := range conns {
				if conn.closed {
					continue
				}

				var ping *keepalive.Ping
				if c.options.KeepaliveClientNewPing != nil {
					ping = c.options.KeepaliveClientNewPing()
				} else {
					ping = &keepalive.Ping{}
				}
				pong := &keepalive.Pong{}
				err := conn.GetProxy(methodName).Call(ctx, ping, pong)
				if c.options.KeepaliveClientHandler != nil {
					c.options.KeepaliveClientHandler(conn, pong, err)
				}
				if err != nil {
					log.Printf("connection: %s keepalive error: %v", conn.peer, err)
					conn.Close()
				}
			}

		case <-c.stopped:
			conns := c.GetConns()
			for _, conn := range conns {
				conn.Close()
			}
			break
		}
	}
}
