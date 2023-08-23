// Package rpc
package rpc

import (
	"testing"

	"github.com/yingshulu/wsrpc/transport"
)

func TestMessage(t *testing.T) {
	var req = "hello, wsrpc"
	f := transport.NewFrame([]byte(req))
	f.SetNext()
	t.Log("length: ", f.Length, ", hasNext: ", f.HasNext(), ", checkSum: ", f.CheckSum)
	data := f.Encode()

	f0 := transport.NewFrame(nil)
	err := f0.Decode(data)
	if err == nil {
		t.Log("length: ", f0.Length, ", hasNext: ", f0.HasNext(), ", checkSum: ", f0.CheckSum)
		t.Log(string(f0.Payload))
	} else {
		t.Log(err)
	}
}
