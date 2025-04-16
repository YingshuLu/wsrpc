package service

import (
	"context"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type ChatService interface {
	SendMessage(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ReceiveMessage(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}
