package frame

import (
	"github.com/yingshulu/wsrpc/transport"
)

func NewQueue() Queue {
	return &list{
		head: newNode(nil, nil),
	}
}

type Queue interface {
	// Push frame into sorted queue
	// any duplicated frame will be discarded
	Push(frame *transport.Frame)

	// Peek next available frame
	Peek() *transport.Frame

	// Pop next available frame
	Pop() *transport.Frame

	// LastPopped get last popped frame
	LastPopped() *transport.Frame
}

type node struct {
	next  *node
	frame *transport.Frame
}

func newNode(f *transport.Frame, next *node) *node {
	return &node{
		frame: f,
		next:  next,
	}
}

type list struct {
	head   *node
	popped *node
}

func (l *list) Push(f *transport.Frame) {
	// duplicated frame, discard
	if l.popped != nil && l.popped.frame.Index >= f.Index {
		return
	}
	curr := l.head.next
	prev := l.head

	for curr != nil {
		if f.Index < curr.frame.Index {
			prev.next = newNode(f, curr)
			return
		} else if f.Index == curr.frame.Index {
			// duplicated frame, discard
			return
		} else {
			prev = curr
			curr = curr.next
		}
	}

	prev.next = newNode(f, nil)
}

func (l *list) first() *node {
	return l.head.next
}

func (l *list) Peek() *transport.Frame {
	poppedIndex := uint16(0)
	if l.popped != nil {
		poppedIndex = l.popped.frame.Index
	}
	if l.first() != nil && l.first().frame.Index == poppedIndex+1 {
		return l.first().frame
	}
	return nil
}

func (l *list) Pop() *transport.Frame {
	if f := l.first(); f != nil {
		l.popped = l.first()
		l.head.next = l.first().next
		return f.frame
	}
	return nil
}

func (l *list) LastPopped() *transport.Frame {
	if l.popped != nil {
		return l.popped.frame
	}
	return nil
}
