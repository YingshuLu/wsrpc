package service

import (
	"context"
	"github.com/yingshulu/wsrpc/rpc"
)

type ChatRoomClient struct {
	conn *rpc.Conn
}

func NewChatRoomClient(conn *rpc.Conn) *ChatRoomClient {
	return &ChatRoomClient{conn: conn}
}

func (c *ChatRoomClient) SendMessage(ctx context.Context, req *ChatRequest, options ...rpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	proxy := c.conn.NewProxy("service.ChatRoom.SendMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func (c *ChatRoomClient) ReceiveMessage(ctx context.Context, req *ChatRequest, options ...rpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	proxy := c.conn.NewProxy("service.ChatRoom.ReceiveMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func RegisterChatRoomService(conn *rpc.Conn, service ChatRoom, options ...rpc.Option) {
	conn.RegisterService("service.ChatRoom", service, options...)
}
