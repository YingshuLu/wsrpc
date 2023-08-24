// Package rpc
package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"github.com/yingshulu/wsrpc/codec"
	"github.com/yingshulu/wsrpc/transport"
)

func newConn(t transport.Transport, holder ServiceHolder, peer string, id string, isClient bool) *Conn {
	c := &Conn{
		t:             t,
		peer:          peer,
		id:            id,
		holder:        holder,
		sendingFrames: make(chan *transport.Frame),
		closeNotify:   make(chan interface{}),
		isClient:      isClient,
	}

	c.log = log.WithFields(log.Fields{
		"Name":     "Connection",
		"ID":       c.id,
		"Peer":     c.peer,
		"IsClient": c.isClient,
		"IsClosed": c.closed,
	})

	c.run(context.Background())
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
	closeNotify   chan interface{}
	closed        bool
	holder        ServiceHolder
	addr          *Addr
	log           *log.Entry
}

func (co *Conn) Peer() string {
	return co.peer
}

func (co *Conn) ID() string {
	return co.id
}

func (co *Conn) IsClosed() bool {
	return co.closed
}

func (co *Conn) GetProxy(name string, options ...Option) Proxy {
	return newProxy(co, strings.ToLower(name), options...)
}

func (co *Conn) run(ctx context.Context) {
	go co.handleRpc(ctx)
	go co.writeFrameTask()
}

func (co *Conn) nextMessageID() uint32 {
	return atomic.AddUint32(&co.messageID, 1)
}

// call client RpcCall
func (co *Conn) call(ctx context.Context, service string, req interface{}, reply interface{}, options *Options) error {
	m := &Message{
		Type:    RequestType,
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
	co.writeFrame(transport.NewFrame(payload))

	ctx, cancel := context.WithTimeout(ctx, options.ServiceTimeout)
	defer func() {
		cancel()
		co.chanMap.Delete(m.ID)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("context done: %v", ctx.Err())

	case <-co.closeNotify:
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

func (co *Conn) handleRpc(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		co.Close()
	}()

	co.log.Println("start reading!")
	for !co.closed {
		co.log.Println("reading frame!")
		fm, err := co.t.Read()
		if err != nil {
			co.log.Println("read err: ", err)
			return err
		}
		co.log.Println("read ok!")

		msg, err := co.getMessage(fm)
		if err != nil {
			co.log.Println("get message err: ", err)
			return err
		}

		switch msg.Type {
		case RequestType:
			go func() {
				co.log.Println("invoke: ", msg.Service)
				replyMsg := co.invokeService(ctx, msg)
				payload, _ := replyMsg.Encode()
				rm := transport.NewFrame(payload)
				co.writeFrame(rm)
			}()

		case ReplyType, ErrorType:
			v, ok := co.chanMap.Load(msg.ID)
			if ok {
				v.(chan *Message) <- msg
			}
		case CloseType:
			co.log.Println("get close msg")
			return co.Close()
		}
	}

	return nil
}

func (co *Conn) invokeService(ctx context.Context, msg *Message) (replyMsg *Message) {
	replyMsg = &Message{}
	var err error
	defer func() {
		replyMsg.Type = ReplyType
		replyMsg.ID = msg.ID
		if err != nil {
			replyMsg.Type = ErrorType
			replyMsg.Error = err.Error()
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
	replyMsg.Data, err = s.Invoke(ctx, mname, msg.Data)
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
				co.log.Println("write err: ", err)
				co.Close()
				return
			}
			co.log.Println("send ok!")
		case <-co.closeNotify:
			break
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
	co.log.Println("connection: ", co.peer, " closed")
	co.holder.RemoveConn(co)
	co.closeNotify <- null
	co.closed = true
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
	co.writeFrame(transport.NewFrame(payload))
}
