// Package rpc
package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/yingshulu/wsrpc/codec"
	"github.com/yingshulu/wsrpc/stream"
	"github.com/yingshulu/wsrpc/transport"
)

func newConn(t transport.Transport, holder ServiceHolder, peer string, id string, isClient bool, header http.Header) *Conn {
	c := &Conn{
		t:             t,
		peer:          peer,
		id:            id,
		holder:        holder,
		sendingFrames: make(chan *transport.Frame, 100),
		isClient:      isClient,
		header:        header,
	}
	c.closedContext, c.closedNotify = context.WithCancel(context.Background())
	c.central = stream.NewCentral(c.closedContext, c.writeFrame)

	c.log = log.WithFields(log.Fields{
		"Name":     "Connection",
		"Host":     holder.HostId(),
		"Peer":     c.peer,
		"IsClient": c.isClient,
	})

	c.run(c.closedContext)
	return c
}

type Conn struct {
	t             transport.Transport
	peer          string
	id            string
	chanMap       sync.Map
	sendingFrames chan *transport.Frame
	messageID     uint32
	isClient      bool
	closed        bool
	holder        ServiceHolder
	addr          string
	header        http.Header
	values        sync.Map
	central       stream.Central
	closedContext context.Context
	closedNotify  context.CancelFunc
	log           *log.Entry
}

func (co *Conn) Host() string {
	return co.holder.HostId()
}

func (co *Conn) Peer() string {
	return co.peer
}

func (co *Conn) ID() string {
	return co.id
}

func (co *Conn) StreamCentral() stream.Central {
	return co.central
}

func (co *Conn) IsClosed() bool {
	return co.closed
}

func (co *Conn) IsClient() bool {
	return co.isClient
}

func (co *Conn) Addr() string {
	return co.addr
}

func (co *Conn) HeaderValue(key string) string {
	if co.header != nil {
		return co.header.Get(key)
	}
	return ""
}

func (co *Conn) GetProxy(name string, options ...Option) Proxy {
	return newProxy(co, strings.ToLower(name), options...)
}

func (co *Conn) WithValue(k string, v interface{}) {
	co.values.Store(k, v)
}

func (co *Conn) Value(k string) interface{} {
	v, ok := co.values.Load(k)
	if ok {
		return v
	}
	return nil
}

func (co *Conn) run(ctx context.Context) {
	go co.handleFrame(ctx)
	go co.writeFrameTask()
}

func (co *Conn) nextMessageID() uint32 {
	return atomic.AddUint32(&co.messageID, 1)
}

// call client RpcCall
func (co *Conn) call(ctx context.Context, service string, req interface{}, reply interface{}, options *Options) error {
	serialize := codec.TypeOf(options.SerializationType)
	if !serialize.Legal() {
		return fmt.Errorf("not support codec type: %v", serialize)
	}

	m := &Message{
		Type:    RequestType,
		Codec:   uint8(serialize),
		ID:      co.nextMessageID(),
		Service: service,
	}

	data, err := codec.Marshal(options.SerializationType, req)
	if err != nil {
		return err
	}
	m.Data = data
	payload, err := m.Encode()
	if err != nil {
		return err
	}

	ch := make(chan *Message, 1)
	co.chanMap.Store(m.ID, ch)
	co.writeFrame(transport.NewRpcFrame(payload))

	ctx, cancel := context.WithTimeout(ctx, options.ServiceTimeout)
	defer func() {
		cancel()
		co.chanMap.Delete(m.ID)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("context done: %v", ctx.Err())

	case <-co.closedContext.Done():
		return fmt.Errorf("connection %s closed", co.peer)

	case replyMsg := <-ch:
		if replyMsg.Type == ErrorType {
			return errors.New(replyMsg.Error)
		}
		err := codec.Unmarshal(options.SerializationType, replyMsg.Data, reply)
		if err != nil {
			return err
		}
	}
	return nil
}

func (co *Conn) handleFrame(ctx context.Context) error {
	defer func() {
		co.Close()
	}()

	for {
		fm, err := co.t.Read()
		if err != nil {
			co.log.Errorf("read err: %v ", err)
			return err
		}

		if fm.Flag&transport.BinFlag != 0 {
			co.central.DispatchFrame(fm)
			continue
		}

		msg, err := co.getMessage(fm)
		if err != nil {
			co.log.Errorf("get message err: %v", err)
			return err
		}

		switch msg.Type {
		case RequestType:
			go func() {
				replyMsg := co.invokeService(ctx, msg)
				payload, _ := replyMsg.Encode()
				rm := transport.NewRpcFrame(payload)
				co.writeFrame(rm)
			}()

		case ReplyType, ErrorType:
			v, ok := co.chanMap.Load(msg.ID)
			if ok {
				v.(chan *Message) <- msg
			}

		case CloseType:
			co.log.Warn("get close msg")
			return nil
		}
	}
}

func (co *Conn) invokeService(ctx context.Context, msg *Message) (replyMsg *Message) {
	var err error
	defer func() {
		if err != nil {
			replyMsg = &Message{
				Type:  ErrorType,
				Error: err.Error(),
			}
		}
	}()

	index := strings.LastIndexByte(msg.Service, '.')
	if index <= 0 {
		err = fmt.Errorf("service %s invalid name", msg.Service)
		return
	}
	sname := msg.Service[0:index]
	mname := msg.Service[index+1:]

	s := co.holder.GetService(sname)
	if s == nil {
		err = fmt.Errorf("service %s not registered", msg.Service)
		return
	}

	ctx = setServiceHolder(ctx, co.holder)
	ctx = setConn(ctx, co)
	replyMsg = s.Invoke(ctx, mname, msg)
	return
}

func (co *Conn) getMessage(fm *transport.Frame) (*Message, error) {
	m := &Message{}
	err := m.Decode(fm.Payload)
	return m, err
}

func (co *Conn) writeFrameTask() {
	for {
		select {
		case frame := <-co.sendingFrames:
			err := co.t.Write(frame)
			if err != nil {
				co.log.Errorf("write err: %v", err)
				co.Close()
				return
			}
		case <-co.closedContext.Done():
			return
		}
	}
}

func (co *Conn) writeFrame(frame *transport.Frame) {
	co.sendingFrames <- frame
}

func (co *Conn) Close() error {
	if co.closed {
		return nil
	}
	co.log.Warn("connection closing")

	defer func() {
		co.closed = true
		co.closedNotify()
		co.holder.RemoveConn(co)
		if options := co.holder.Options(); options != nil && options.ConnectionClosedEvent != nil {
			go options.ConnectionClosedEvent(co)
		}
		co.central.Stop()
	}()

	return co.t.Close()
}

func (co *Conn) CloseMessage(message string) {
	if co.closed {
		return
	}

	msg := &Message{
		Type:  CloseType,
		Error: message,
	}
	payload, _ := msg.Encode()
	co.writeFrame(transport.NewRpcFrame(payload))
}
