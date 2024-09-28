// Package rpc
package rpc

import (
	"net/http"
	"time"
)

type Option = func(*Options)

type CredentialValidator = func(string) error

type CredentialProvider = func() string

type OnConnectionEstablishedEvent = func(*Conn)

type OnConnectionClosedEvent = func(*Conn)

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

func WithOnConnectionEstablishedEvent(e OnConnectionEstablishedEvent) Option {
	return func(op *Options) {
		op.ConnectionEstablishedEvent = e
	}
}

func WithOnConnectionClosedEvent(e OnConnectionClosedEvent) Option {
	return func(op *Options) {
		op.ConnectionClosedEvent = e
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

func WithRequestHeader(header http.Header) Option {
	return func(op *Options) {
		op.RequestHeader = header
	}
}

func WithResponseHeader(header http.Header) Option {
	return func(op *Options) {
		op.ResponseHeader = header
	}
}

func WithEnableStream(enable bool) Option {
	return func(op *Options) {
		op.EnableStream = enable
	}
}

type Options struct {
	ServiceTimeout             time.Duration
	SerializationType          string
	RequestHeader              http.Header
	ResponseHeader             http.Header
	EnableStream               bool
	CredentialProvider         CredentialProvider
	CredentialValidator        CredentialValidator
	ConnectionEstablishedEvent OnConnectionEstablishedEvent
	ConnectionClosedEvent      OnConnectionClosedEvent
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
		RequestHeader:     http.Header{},
		ResponseHeader:    http.Header{},
	}
}
