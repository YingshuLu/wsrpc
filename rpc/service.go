// Package rpc
package rpc

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/yingshulu/wsrpc/codec"
)

// func (s *helloService) Say(ctx context.Context, req *HelloRequest, reply *HelloReply) error

var (
	typeOfError   = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()
)

type Service interface {
	Invoke(ctx context.Context, methodName string, req *Message) (reply *Message)
}

func newService(name string, receiver interface{}, options ...Option) Service {
	s := new(service)
	s.name = name
	s.methods = map[string]*method{}
	s.receiver = reflect.ValueOf(receiver)
	s.receiverType = reflect.TypeOf(receiver)
	s.options = defaultOptions()
	Assert(s.receiverType.Kind() == reflect.Ptr, fmt.Sprintf("%s not pointer type", s.receiverType.Name()))

	typ := s.receiverType
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if !suitableMethod(m) {
			continue
		}
		mt := m.Type
		method := new(method)
		method.name = strings.ToLower(m.Name)
		method.requestType = mt.In(2)
		method.replyType = mt.In(3)
		method.proc = m
		s.methods[method.name] = method
	}
	s.options.Apply(options)
	return s
}

type service struct {
	name         string
	receiver     reflect.Value
	receiverType reflect.Type
	methods      map[string]*method
	options      *Options
}

func (s *service) Invoke(ctx context.Context, methodName string, reqMessage *Message) (replyMessage *Message) {
	replyMessage = &Message{}
	var err error
	defer func() {
		if err != nil {
			replyMessage.Type = ErrorType
			replyMessage.Error = err.Error()
		} else {
			replyMessage.Type = ReplyType
			replyMessage.Codec = reqMessage.Codec
			replyMessage.ID = reqMessage.ID
		}
	}()

	serialize := codec.Type(reqMessage.Codec)
	if !serialize.Legal() {
		err = fmt.Errorf("not support codec type: %v", serialize)
		return
	}

	m, ok := s.methods[methodName]
	if !ok {
		err = fmt.Errorf("not found service %s.%s", s.name, methodName)
		return
	}

	req, reply := m.newExchangeValues()
	err = codec.Unmarshal(serialize.String(), reqMessage.Data, req.Interface())
	if err != nil {
		return
	}

	// call service
	{
		notify := make(chan error, 1)
		ctx, cancel := context.WithTimeout(ctx, s.options.ServiceTimeout)
		defer cancel()

		go func() {
			var invokeErr error
			defer func() {
				if r := recover(); r != nil {
					invokeErr = fmt.Errorf("call service %s.%s panic: %v", s.name, methodName, r)
				}
				notify <- invokeErr
			}()

			invokeErr = m.invoke(ctx, s.receiver, req, reply)
		}()

		select {
		case v := <-ctx.Done():
			err = fmt.Errorf("call service %s.%s context done: %v", s.name, methodName, v)
		case err = <-notify:
		}

		if err != nil {
			return
		}
	}

	replyMessage.Data, err = codec.Marshal(serialize.String(), reply.Interface())
	return
}

type method struct {
	name        string
	requestType reflect.Type
	replyType   reflect.Type
	proc        reflect.Method
}

func (m *method) invoke(ctx context.Context, receiver, request, reply reflect.Value) error {
	values := m.proc.Func.Call([]reflect.Value{receiver, reflect.ValueOf(ctx), request, reply})
	err := values[0].Interface()
	if err == nil {
		return nil
	}
	return err.(error)
}

func (m *method) newExchangeValues() (request, reply reflect.Value) {
	return m.newValue(m.requestType), m.newValue(m.replyType)
}

func (m *method) newValue(typ reflect.Type) (value reflect.Value) {
	return reflect.New(typ.Elem())
}

func isExport(name string) bool {
	c := name[0]
	return c >= 'A' && c <= 'Z'
}

func suitableMethod(m reflect.Method) bool {
	if !isExport(m.Name) {
		return false
	}

	mt := m.Type
	return mt.NumIn() == 4 &&
		mt.NumOut() == 1 &&
		mt.In(0).Kind() == reflect.Ptr &&
		mt.In(1) == typeOfContext &&
		mt.In(2).Kind() == reflect.Ptr &&
		mt.In(3).Kind() == reflect.Ptr &&
		mt.Out(0) == typeOfError
}
