// Package rpc
package rpc

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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
	return c
}

type Client struct {
	ServiceHolder
	Host    string
	stopped chan interface{}
	options *Options
}

func (c *Client) Options() *Options {
	return c.options
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
	if oldConn := c.GetConnByPeer(conn.Peer()); oldConn != nil {
		oldConn.Close()
	}
	c.AddConn(conn)
	if c.options.ConnectionEstablishedEvent != nil {
		c.options.ConnectionClosedEvent(conn)
	}
}
