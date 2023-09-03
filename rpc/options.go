// Package rpc
package rpc

import (
	"time"

	"github.com/yingshulu/wsrpc/rpc/service/keepalive"
)

const defaultKeepaliveTimeout = 20 * time.Minute

type Option = func(*Options)

type CredentialValidator = func(string) error

type CredentialProvider = func() string

// WithCredentialProvider set credential provider for client when connecting to server
func WithCredentialProvider(f CredentialProvider) Option {
	return func(op *Options) {
		op.CredentialProvider = f
	}
}

// WithCredentialValidator set credential validator for server when accepting from client
func WithCredentialValidator(f CredentialValidator) Option {
	return func(op *Options) {
		op.CredentialValidator = f
	}
}

func WithServiceTimeout(d time.Duration) Option {
	return func(op *Options) {
		op.ServiceTimeout = d
	}
}

func WithSerialization(e string) Option {
	return func(op *Options) {
		op.SerializationType = e
	}
}

func WithKeepaliveTimeout(d time.Duration) Option {
	return func(op *Options) {
		op.KeepaliveTimeout = d
	}
}

func WithKeepaliveClientNewPing(f func() *keepalive.Ping) Option {
	return func(op *Options) {
		op.KeepaliveClientNewPing = f
	}
}

func WithKeepaliveClientHandler(f func(*Conn, *keepalive.Pong, error)) Option {
	return func(op *Options) {
		op.KeepaliveClientHandler = f
	}
}

func WithKeepaliveServerHandler(f keepalive.Handler) Option {
	return func(op *Options) {
		op.KeepaliveServerHandler = f
	}
}

type Options struct {
	KeepaliveClientNewPing func() *keepalive.Ping
	KeepaliveClientHandler func(*Conn, *keepalive.Pong, error)
	KeepaliveServerHandler keepalive.Handler
	KeepaliveTimeout       time.Duration
	ServiceTimeout         time.Duration
	SerializationType      string
	CredentialProvider     CredentialProvider
	CredentialValidator    CredentialValidator
}

func (op *Options) Apply(options []Option) {
	for _, f := range options {
		f(op)
	}
}

func defaultOptions() *Options {
	return &Options{
		KeepaliveTimeout:  defaultKeepaliveTimeout,
		ServiceTimeout:    5 * time.Second,
		SerializationType: "json",
	}
}
