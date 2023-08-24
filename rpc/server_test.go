// Package rpc
package rpc

import (
	"testing"
)

func TestServer_Run(t *testing.T) {
	s := NewServer("test")
	s.AddService("testservice", &TestService{},
		WithServiceTimeout(1000),
		WithSerialization("json"))

	s.RunTcp(":8443")
}
