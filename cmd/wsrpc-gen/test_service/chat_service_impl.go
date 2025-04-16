package service

import (
	"context"
	"github.com/yingshulu/wsrpc"
)

type ChatServiceClient struct {
	conn *wsrpc.Conn
}

func NewChatServiceClient(conn *wsrpc.Conn) *ChatServiceClient {
	return &ChatServiceClient{conn: conn}
}

func (c *ChatServiceClient) SendMessage(ctx context.Context, req *ChatRequest, options ...wsrpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	proxy := c.conn.NewProxy("service.ChatService.SendMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func (c *ChatServiceClient) ReceiveMessage(ctx context.Context, req *ChatRequest, options ...wsrpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	proxy := c.conn.NewProxy("service.ChatService.ReceiveMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func RegisterChatService(conn *wsrpc.Conn, service ChatService, options ...wsrpc.Option) {
	conn.RegisterService("service.ChatService", service, options...)
}
