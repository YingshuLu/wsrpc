// Package rpc
package rpc

import (
	"context"
	"strings"
	"sync"
)

const contextHolderKey = "_holder_"

func GetServiceHolder(ctx context.Context) ServiceHolder {
	v := ctx.Value(contextHolderKey)
	if v == nil {
		return nil
	}
	return v.(ServiceHolder)
}

func setServiceHolder(ctx context.Context, h ServiceHolder) context.Context {
	return context.WithValue(ctx, contextHolderKey, h)
}

func newServiceHolder() ServiceHolder {
	return &serviceHolder{
		services:     map[string]Service{},
		keyIdConns:   map[string]*Conn{},
		keyPeerConns: map[string]*Conn{},
	}
}

type ServiceHolder interface {
	AddService(name string, impl interface{}, options ...Option)
	GetService(name string) Service
	GetConnById(id string) *Conn
	GetConnByPeer(peer string) *Conn
	GetConns() []*Conn
	AddConn(conn *Conn)
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
