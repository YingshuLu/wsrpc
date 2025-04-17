package service

import (
	"context"
	"fmt"

	"github.com/yingshulu/wsrpc/rpc"
)

type ChatRoomClient struct {
	h    rpc.ServiceHolder
	peer string
}

func NewChatRoomClient(h rpc.ServiceHolder, peer string) *ChatRoomClient {
	return &ChatRoomClient{h: h, peer: peer}
}

func (c *ChatRoomClient) SendMessage(ctx context.Context, req *ChatRequest, options ...rpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	conn := c.h.GetConnByPeer(c.peer)
	if conn == nil {
		return res, fmt.Errorf("c.peer connection not found")
	}
	proxy := conn.GetProxy("service.ChatRoom.SendMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func (c *ChatRoomClient) ReceiveMessage(ctx context.Context, req *ChatRequest, options ...rpc.Option) (*ChatResponse, error) {
	var res *ChatResponse
	conn := c.h.GetConnByPeer(c.peer)
	if conn == nil {
		return res, fmt.Errorf("c.peer connection not found")
	}
	proxy := conn.GetProxy("service.ChatRoom.ReceiveMessage")
	err := proxy.Call(ctx, req, res, options...)
	return res, err
}

func RegisterChatRoomService(service ChatRoom, options ...rpc.Option) {
	rpc.RegisterService("service.ChatRoom", service, options...)
}
