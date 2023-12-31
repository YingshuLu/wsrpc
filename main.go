// Package wsrpc
package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc"
)

type Hello struct {
	Something string
}

type Reply struct {
	Message string
}

type TestService struct {
}

func (t *TestService) Say(ctx context.Context, h *Hello, reply *Reply) error {
	reply.Message = "reply: " + h.Something
	return nil
}

func (t *TestService) PanicSay(ctx context.Context, h *Hello, reply *Reply) error {
	return nil
}

func (t *TestService) wisp(ctx context.Context, h *Hello, reply *Reply) error {
	return nil
}

func (t *TestService) Add(v int) int {
	return v + 1
}

func main() {
	s := rpc.NewServer("test", rpc.WithCredentialValidator(func(s string) error {
		return nil
	}))
	s.AddService("testservice", &TestService{},
		rpc.WithServiceTimeout(10*time.Second),
		rpc.WithSerialization("json"))

	go func() {
		var conn *rpc.Conn
		for conn == nil {
			time.Sleep(1 * time.Second)
			log.Println("wait connection...")
			conn = s.GetConnByPeer("macbook")
		}
		proxy := conn.GetProxy("testservice.say", rpc.WithSerialization("json"), rpc.WithServiceTimeout(10*time.Second))

		for i := 0; i < 10; i++ {
			reply := &Reply{}
			err := proxy.Call(context.Background(), &Hello{"from macbook"}, reply)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("client: ", reply)
			}
		}
	}()

	go s.RunWs("ws://+:9090/websocket")
	s.RunTcp(":8443")

}
