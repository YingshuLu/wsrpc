// Package rpc
package rpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
	stopped chan interface{}
	options *Options
}

func (c *Client) Connect(a string) error {
	u, err := url.Parse(a)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "tcp":
		return c.dialConnect(a, u.Scheme)
	case "ws", "wss":
		return c.wsConnect(a)
	}
	return fmt.Errorf("not support connection: %v", a)
}

func (c *Client) wsConnect(a string) error {
	header := http.Header{}
	header.Add(hostIdKey, c.Host)
	if c.options.CredentialProvider != nil {
		header.Add(authKey, c.options.CredentialProvider())
	}
	wsc, resp, err := websocket.DefaultDialer.Dial(a, header)
	if err != nil {
		return err
	}

	id := resp.Header.Get(connIdKey)
	peer := resp.Header.Get(hostIdKey)
	c.onConnected(transport.NewWebSocket(wsc), peer, id, a)
	return nil
}

func (c *Client) dialConnect(a string, typ string) error {
	u, _ := url.Parse(a)
	tc, err := net.Dial(typ, u.Host)
	if err != nil {
		return err
	}

	var secret string
	if c.options.CredentialProvider != nil {
		secret = c.options.CredentialProvider()
	}
	peer, id, err := transport.ClientNegotiate(tc, c.Host, secret)
	if err != nil {
		log.Printf("client negotiate error: %s", err)
		return err
	}
	c.onConnected(transport.New(tc), peer, id, a)
	return nil
}

func (c *Client) Close() {
	c.stopped <- null
}

func (c *Client) onConnected(t transport.Transport, peer, id string, a string) {
	conn := newConn(t, c, peer, id, true)
	conn.addr = a
	c.AddConn(conn)
}

func (c *Client) scheduleKeepalive() {
	methodName := keepalive.ServiceName + ".keepalive"
	ctx := context.Background()

	clientStopped := false
	for !clientStopped {
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
			clientStopped = true
		}
	}
}
