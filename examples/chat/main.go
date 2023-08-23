package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/rpc"
)

func serverRun() {
	s := rpc.NewServer("exchange", 20*time.Second)
	s.AddService("chat.exchange", &Exchange{})
	s.RunWs(":8443")
}

func client(host string) *rpc.Client {
	c := rpc.NewClient(host, 1*time.Second)
	c.AddService("chat.receiver", &Receiver{})
	c.WsConnect("ws://localhost:8443/websocket")
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
