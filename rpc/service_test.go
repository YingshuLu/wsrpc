// Package wsrpc
package rpc

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type Hello struct {
	Something string
}

type Reply struct {
	Message string
}

type TestService struct {
}

func (t *TestService) Say(_ context.Context, h *Hello, reply *Reply) error {
	reply.Message = "reply: " + h.Something
	return nil
}

func (t *TestService) PanicSay(_ context.Context, h *Hello, reply *Reply) error {
	panic("test panic: " + h.Something)
}

func (t *TestService) wisp(_ context.Context, _ *Hello, _ *Reply) error {
	return nil
}

func (t *TestService) Add(v int) int {
	return v + 1
}

func TestBuildService(t *testing.T) {
	s := newService("TestService", &TestService{})
	ss := s.(*service)

	for name := range ss.methods {
		t.Log(name)
	}
	m := ss.methods["say"]

	_, ok := ss.methods["wisp"]
	Assert(!ok, "error method")

	h := &Hello{"Hello wsrpc!"}
	r := Reply{}

	m.invoke(context.Background(), ss.receiver, reflect.ValueOf(h), reflect.ValueOf(&r))
	fmt.Println(r)
}
