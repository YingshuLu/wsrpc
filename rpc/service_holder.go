// Package rpc
package rpc

import (
	"context"
	"strings"
	"sync"
)

const (
	contextHolderKey = "_holder_"
	contextConnKey   = "_self_"
)

func GetServiceHolder(ctx context.Context) ServiceHolder {
	v := ctx.Value(contextHolderKey)
	if v == nil {
		return nil
	}
	return v.(ServiceHolder)
}

func GetConn(ctx context.Context) *Conn {
	if v := ctx.Value(contextConnKey); v != nil {
		return v.(*Conn)
	}
	return nil
}

func setServiceHolder(ctx context.Context, h ServiceHolder) context.Context {
	return context.WithValue(ctx, contextHolderKey, h)
}

func setConn(ctx context.Context, c *Conn) context.Context {
	return context.WithValue(ctx, contextConnKey, c)
}

func newServiceHolder() ServiceHolder {
	return &serviceHolder{
		services:     map[string]Service{},
		keyIdConns:   map[string]*Conn{},
		keyPeerConns: map[string]*Conn{},
	}
}

type ServiceHolder interface {
	// AddService add service with name
	AddService(name string, impl interface{}, options ...Option)

	// GetService get service by name
	GetService(name string) Service

	// GetConnById get connection by connection id
	GetConnById(id string) *Conn

	// GetConnByPeer get connection by connection peer
	GetConnByPeer(peer string) *Conn

	// GetConns get all available connections
	GetConns() []*Conn

	// AddConn add a connection into holder
	AddConn(conn *Conn)

	// RemoveConn remove connection from holder
	RemoveConn(conn *Conn)
}

type serviceHolder struct {
	services     map[string]Service
	lock         sync.RWMutex
	keyIdConns   map[string]*Conn
	keyPeerConns map[string]*Conn
}

func (s *serviceHolder) GetService(name string) Service {
	return s.services[strings.ToLower(name)]
}

func (s *serviceHolder) AddService(name string, impl interface{}, options ...Option) {
	svc := newService(name, impl, options...)
	s.services[strings.ToLower(name)] = svc
}

func (s *serviceHolder) AddConn(c *Conn) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.keyIdConns[c.id] = c
	s.keyPeerConns[c.peer] = c
}

func (s *serviceHolder) RemoveConn(c *Conn) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.keyIdConns, c.id)
	delete(s.keyPeerConns, c.peer)
}

func (s *serviceHolder) GetConnById(id string) *Conn {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.keyIdConns[id]
}

func (s *serviceHolder) GetConnByPeer(peer string) *Conn {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.keyPeerConns[peer]
}

func (s *serviceHolder) GetConns() []*Conn {
	s.lock.RLock()
	defer s.lock.RUnlock()

	n := len(s.keyIdConns)
	conns := make([]*Conn, 0, n)
	for _, c := range s.keyIdConns {
		conns = append(conns, c)
	}
	return conns
}
