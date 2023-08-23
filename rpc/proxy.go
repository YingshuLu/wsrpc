// Package rpc
package rpc

import (
	"context"
)

func newProxy(conn *Conn, name string, options ...Option) Proxy {
	p := &serviceProxy{
		serviceMethod: name,
		options:       DefaultOptions(),
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
	var ops *Options
	if len(options) == 0 {
		ops = p.options
	} else {
		ops = new(Options)
		for _, f := range options {
			f(ops)
		}
	}
	return p.conn.call(ctx, p.serviceMethod, request, reply, ops)
}
