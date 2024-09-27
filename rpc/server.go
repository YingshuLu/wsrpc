// Package rpc
package rpc

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/transport"
)

const (
	connIdKey = "X-CONNECTION-ID"
	hostIdKey = "X-HOST-ID"
	authKey   = "X-AUTH-TOKEN"
)

func NewServer(host string, options ...Option) *Server {
	s := &Server{
		Host:          host,
		ServiceHolder: newServiceHolder(),
		ws:            &websocket.Upgrader{},
		options:       defaultOptions(),
	}
	s.options.Apply(options)
	return s
}

type Server struct {
	ServiceHolder
	Host    string
	ws      *websocket.Upgrader
	addr    string
	State   int
	options *Options
}

func (s *Server) Options() *Options {
	return s.options
}

func (s *Server) RegisterWs(path string, mux *http.ServeMux) {
	mux.HandleFunc(path, s.wsAccept)
}

func (s *Server) RunWs(addr string) {
	s.addr = addr
	u, err := url.Parse(s.addr)
	if err != nil {
		log.Fatal(err)
	}

	port := u.Port()
	if len(port) == 0 {
		switch u.Scheme {
		case "ws":
			port = ":80"
		case "wss":
			port = ":443"
		default:
			log.Fatalf("url %s is invalid ", addr)
		}
	} else {
		port = ":" + port
	}

	hs := http.NewServeMux()
	s.RegisterWs(u.Path, hs)
	log.Fatal(http.ListenAndServe(port, hs))
}

func (s *Server) wsAccept(w http.ResponseWriter, r *http.Request) {
	id := uuid.NewString()
	peer := r.Header.Get(hostIdKey)

	// auth client
	if s.options.CredentialValidator != nil {
		secret := r.Header.Get(authKey)
		if err := s.options.CredentialValidator(secret); err != nil {
			w.WriteHeader(http.StatusForbidden)
			reason := fmt.Sprintf("{\"refused\": %v}", err)
			w.Write([]byte(reason))
			return
		}
	}

	header := s.options.ResponseHeader.Clone()
	header.Add(connIdKey, id)
	header.Add(hostIdKey, s.Host)
	wsc, err := s.ws.Upgrade(w, r, header)
	if err != nil {
		log.Println("websocket upgrade error ", err)
		return
	}
	log.Println("accept: ", r.Header)

	conn := newConn(transport.NewWebSocket(wsc), s, peer, id, false, r.Header)
	if oldConn := s.GetConnByPeer(conn.Peer()); oldConn != nil {
		oldConn.Close()
	}
	s.AddConn(conn)
	if s.options.ConnectionEstablishedEvent != nil {
		s.options.ConnectionEstablishedEvent(conn)
	}
}

func (s *Server) RunTcp(addr string) {
	ss, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		tc, err := ss.Accept()
		if err != nil {
			log.Fatal(err)
		}

		id := uuid.NewString()
		peer, err := transport.ServerNegotiate(tc, s.Host, id, s.options.CredentialValidator)
		if err != nil {
			log.Printf("server negotiate error: %s", err)
			tc.Close()
			continue
		}

		conn := newConn(transport.New(tc), s, peer, id, false, nil)
		if oldConn := s.GetConnByPeer(conn.Peer()); oldConn != nil {
			oldConn.Close()
		}
		s.AddConn(conn)
		if s.options.ConnectionEstablishedEvent != nil {
			s.options.ConnectionEstablishedEvent(conn)
		}
	}
}
