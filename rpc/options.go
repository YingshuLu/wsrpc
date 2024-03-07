// Package rpc
package rpc

import (
	"time"
)

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

type Options struct {
	ServiceTimeout      time.Duration
	SerializationType   string
	CredentialProvider  CredentialProvider
	CredentialValidator CredentialValidator
}

func (op *Options) Apply(options []Option) {
	for _, f := range options {
		f(op)
	}
}

func defaultOptions() *Options {
	return &Options{
		ServiceTimeout:    5 * time.Second,
		SerializationType: "json",
	}
}
