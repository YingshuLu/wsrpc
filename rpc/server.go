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

	"github.com/yingshulu/wsrpc/stream"
)

const (
	connIdKey = "X-CONNECTION-ID"
	hostIdKey = "X-HOST-ID"
	authKey   = "X-AUTH-TOKEN"
)

func NewServer(host string, options ...Option) *Server {
	s := &Server{
		Host:          host,
		ServiceHolder: newServiceHolder(host),
		ws: &websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		options: defaultOptions(),
	}
	s.options.Apply(options)
	s.ws.HandshakeTimeout = s.options.ServiceTimeout
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
		log.Error("websocket upgrade error ", err)
		return
	}
	log.Debug("accept: ", r.Header)

	s.onAccept(transport.NewWebSocket(wsc), peer, id, r.Header)
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
			log.Errorf("server negotiate error: %s", err)
			tc.Close()
			continue
		}

		s.onAccept(transport.New(tc), peer, id, nil)
	}
}

func (s *Server) onAccept(t transport.Transport, peer string, id string, header http.Header) {
	var central stream.Central
	if s.options.EnableStream {
		central = stream.NewCentral(t)
		t = central
	}
	conn := newConn(t, s, peer, id, false, header)
	conn.central = central
	if oldConn := s.GetConnByPeer(conn.Peer()); oldConn != nil {
		oldConn.Close()
	}
	s.AddConn(conn)
	if s.options.ConnectionEstablishedEvent != nil {
		s.options.ConnectionEstablishedEvent(conn)
	}
}
