// Package rpc
package rpc

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc/service/keepalive"
	"github.com/yingshulu/wsrpc/transport"
)

func NewClient(host string, timeout time.Duration) *Client {
	c := &Client{
		Host:          host,
		ServiceHolder: newServiceHolder(),
		timeout:       timeout,
		stopped:       make(chan interface{}),
	}
	go c.scheduleKeepalive()
	return c
}

type Client struct {
	ServiceHolder
	Host    string
	addr    string
	timeout time.Duration
	stopped chan interface{}
}

func (c *Client) WsConnect(addr string) error {
	c.addr = addr
	header := http.Header{}
	header.Add(hostIdKey, c.Host)
	wsc, resp, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		return err
	}

	id := resp.Header.Get(connIdKey)
	peer := resp.Header.Get(hostIdKey)
	c.onConnected(transport.NewWebSocket(wsc), peer, id)
	return nil
}

func (c *Client) TcpConnect(addr string) error {
	peer := "test-tcp"
	id := "123"
	tc, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c.onConnected(transport.New(tc), peer, id)
	return nil
}

func (c *Client) Close() {
	c.stopped <- null
}

func (c *Client) onConnected(t transport.Transport, peer, id string) {
	conn := newConn(t, c, peer, id, true)
	c.AddConn(conn)
}

func (c *Client) scheduleKeepalive() {
	methodName := keepalive.ServiceName + ".keepalive"
	ping := &keepalive.Ping{}
	for {
		select {

		case <-time.After(c.timeout):
			conns := c.GetConns()
			for _, conn := range conns {
				if conn.closed {
					continue
				}
				err := conn.GetProxy(methodName).Call(context.Background(), ping, &keepalive.Pong{})
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
