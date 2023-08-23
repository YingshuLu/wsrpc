// Package keepalive
package keepalive

import "context"

const ServiceName = "_rpc.keepalive_"

type Handler func(context.Context, *Ping, *Pong) error

func NewService(h Handler) *Service {
	return &Service{h}
}

type Ping struct {
	Payload string
}

type Pong struct {
	Payload string
}

type Service struct {
	handler Handler
}

func (s *Service) Keepalive(ctx context.Context, ping *Ping, pong *Pong) error {
	if s.handler != nil {
		return s.handler(ctx, ping, pong)
	}
	pong.Payload = ping.Payload
	return nil
}
