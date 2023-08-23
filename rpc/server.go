// Package rpc
package rpc

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yingshulu/wsrpc/rpc/service/keepalive"
	"github.com/yingshulu/wsrpc/transport"
)

const (
	connIdKey = "X-CONNECTION-ID"
	hostIdKey = "X-HOST-ID"
	authKey   = "X-AUTH-TOKEN"
)

const (
	Init = iota
	Running
	Stopped
)

func NewServer(host string, timeout time.Duration) *Server {
	s := &Server{
		Host:          host,
		timeout:       timeout,
		ServiceHolder: newServiceHolder(),
		ws:            &websocket.Upgrader{},
	}
	s.AddService(keepalive.ServiceName, keepalive.NewService(nil))
	return s
}

type Server struct {
	ServiceHolder
	Host    string
	ws      *websocket.Upgrader
	timeout time.Duration
	addr    string
	State   int
}

func (s *Server) RunWs(addr string) {
	s.addr = addr
	http.HandleFunc("/websocket", s.wsAccept)
	log.Fatal(http.ListenAndServe(s.addr, nil))
}

func (s *Server) wsAccept(w http.ResponseWriter, r *http.Request) {
	id := uuid.NewString()
	peer := r.Header.Get(hostIdKey)

	header := http.Header{}
	header.Add(connIdKey, id)
	header.Add(hostIdKey, s.Host)
	wsc, err := s.ws.Upgrade(w, r, header)
	if err != nil {
		log.Println("websocket upgrade error ", err)
		return
	}
	log.Println("accept: ", r.Header)

	conn := newConn(transport.NewWebSocket(wsc), s, peer, id, false)
	s.AddConn(conn)
	return
}

func (s *Server) RunTcp(addr string) {
	ss, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	id := uuid.NewString()
	peer := "macbook-tcp"

	for {
		tc, err := ss.Accept()
		if err != nil {
			log.Fatal(err)
		}
		conn := newConn(transport.New(tc), s, peer, id, false)
		s.AddConn(conn)
	}

}
