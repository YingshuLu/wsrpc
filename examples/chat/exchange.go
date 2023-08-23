package main

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc"
)

type Msg struct {
	Id      string
	Host    string
	Peer    string
	Content string
}

type Ack struct {
	Status  int
	Id      string
	Content string
}

type Exchange struct{}

func (s *Exchange) Exchange(ctx context.Context, msg *Msg, ack *Ack) error {
	holder := rpc.GetServiceHolder(ctx)
	conn := holder.GetConnByPeer(msg.Peer)
	if conn == nil {
		ack.Status = 403
		ack.Id = msg.Id
		ack.Content = fmt.Sprintf("not found peer[%s]", msg.Peer)
		return nil
	}

	proxy := conn.GetProxy("chat.receiver.box")
	err := proxy.Call(ctx, msg, ack)
	if err != nil {
		ack.Status = 500
		ack.Id = msg.Id
		ack.Content = fmt.Sprintf("channel broken peer[%s], cause: %v", msg.Peer, err)
	} else {
		log.Printf("[exchange] get ack from %s", msg.Peer)
	}

	return nil
}

type Receiver struct{}

func (s *Receiver) Box(ctx context.Context, msg *Msg, ack *Ack) error {
	log.Printf("reciver from %s, content: %s", msg.Host, msg.Content)
	ack.Id = msg.Id
	ack.Status = 200
	ack.Content = "petty fine, and u?"
	return nil
}
