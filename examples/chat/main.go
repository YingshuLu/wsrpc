package main

import (
	"context"
	"github.com/yingshulu/wsrpc/rpc/service/keepalive"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc"
)

func serverRun() {
	s := rpc.NewServer("exchange")
	s.AddService("chat.exchange", &Exchange{})
	s.RunWs(":8443")
}

func handleKeepalive(c *rpc.Conn, pong *keepalive.Pong, err error) {
	if err != nil {
		c.Close()
	} else {
		log.Printf("client received connection pong!")
	}
}

func client(host string) *rpc.Client {
	c := rpc.NewClient(host, rpc.WithKeepaliveTimeout(time.Second), rpc.WithKeepaliveClientHandler(handleKeepalive))
	c.AddService("chat.receiver", &Receiver{})
	err := c.Connect(&rpc.Addr{
		Host: "ws://localhost:8443/websocket",
	})
	if err != nil {
		log.Println("connect error: ", err)
	}
	return c
}

func send(c *rpc.Client, peer string) {
	conn := c.GetConnByPeer("exchange")
	msg := &Msg{
		Id:      uuid.NewString(),
		Host:    c.Host,
		Peer:    peer,
		Content: "how are you?",
	}
	ack := &Ack{}

	proxy := conn.GetProxy("chat.exchange.exchange")
	err := proxy.Call(context.Background(), msg, ack)
	if err != nil {
		log.Printf("[%s] send message error: %s", c.Host, err)
		return
	}
	log.Printf("[%s] receive [%s] ack: %s", c.Host, peer, ack.Content)
}

func compose() {
	go serverRun()
	time.Sleep(2 * time.Second)

	kamlu := client("kamlu")
	robot := client("robot")

	go send(robot, kamlu.Host)
	go send(kamlu, robot.Host)
	time.Sleep(10 * time.Second)
	kamlu.Close()
	robot.Close()
}

func main() {
	compose()
	time.Sleep(time.Second)
}
