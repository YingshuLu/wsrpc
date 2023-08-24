// Package rpc
package rpc

import (
	"context"
)

func newProxy(conn *Conn, name string, options ...Option) Proxy {
	p := &serviceProxy{
		serviceMethod: name,
		options:       defaultOptions(),
		conn:          conn,
	}

	p.options.Apply(options)
	return p
}

type Proxy interface {
	Call(ctx context.Context, request, reply interface{}, options ...Option) error
}

type serviceProxy struct {
	serviceMethod string
	options       *Options
	conn          *Conn
}

func (p *serviceProxy) Call(ctx context.Context, request, reply interface{}, options ...Option) error {
	var ops = *p.options
	(&ops).Apply(options)
	return p.conn.call(ctx, p.serviceMethod, request, reply, &ops)
}
