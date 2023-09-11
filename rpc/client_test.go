// Package rpc
package rpc

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("macbook", WithKeepaliveTimeout(1*time.Second))

	c.AddService("testservice", &TestService{},
		WithServiceTimeout(10*time.Second),
		WithSerialization("json"))

	//log.Println(c.Connect("ws://localhost:9090/websocket"))

	err := c.Connect("tcp://localhost:8443")
	if err != nil {
		t.Log(err)
		return
	}

	conn := c.GetConnByPeer("test")
	proxy := conn.GetProxy("testservice.say", WithSerialization("json"), WithServiceTimeout(10*time.Second))

	for i := 0; i < 10; i++ {
		reply := &Reply{}
		err = proxy.Call(context.Background(), &Hello{"from macbook"}, reply)
		if err != nil {
			t.Log(err)
		} else {
			t.Log("server: ", reply.Message)
		}
	}
	time.Sleep(30 * time.Second)
}
