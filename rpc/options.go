// Package rpc
package rpc

import "time"

type Option = func(*Options)

func WithTimeout(d time.Duration) Option {
	return func(op *Options) {
		op.Timeout = d
	}
}

func WithSerialization(e string) Option {
	return func(op *Options) {
		op.SerializationType = e
	}
}

func DefaultOptions() *Options {
	return &Options{
		Timeout:           5 * time.Second,
		SerializationType: "json",
	}
}

type Options struct {
	Timeout           time.Duration
	SerializationType string
}

func (op *Options) Apply(options []Option) {
	for _, f := range options {
		f(op)
	}
}
