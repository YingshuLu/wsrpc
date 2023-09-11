// Package keepalive
package keepalive

import (
	"context"
	log "github.com/sirupsen/logrus"
)

const ServiceName = "_rpc.keepalive_"

type Handler func(context.Context, *Ping, *Pong) error

func NewService(h Handler) *Service {
	return &Service{h}
}

type Service struct {
	handler Handler
}

func (s *Service) Keepalive(ctx context.Context, ping *Ping, pong *Pong) error {
	if s.handler != nil {
		return s.handler(ctx, ping, pong)
	}
	pong.Content = ping.Content
	log.Printf("in keepalive service")
	return nil
}
