package main

import (
	"context"
	"fmt"

	"github.com/yingshulu/wsrpc/rpc"
)

type EchoService struct {
}

func (e *EchoService) Serve(ctx context.Context, request *Ping, reply *Pong) error {
	fmt.Println("from request: ", request.Content)
	reply.Id = request.Id

	var conn = rpc.GetConn(ctx)
	var proxy = conn.GetProxy("misty.echo.serve", rpc.WithSerialization("protobuf"))
	request.Content = "ping from go-wsrpc"
	err := proxy.Call(ctx, request, reply)
	if err == nil {
		fmt.Println("reply from swift:", reply.Content)
	}

	reply.Content += ", this go wsrpc echo service"
	return err
}

func main() {
	s := rpc.NewServer("misty")
	s.AddService("misty.echo", &EchoService{}, rpc.WithSerialization("protobuf"))
	s.RunWs("ws://+:8080/websocket")
}
