// Package rpc
package rpc

import "testing"

func TestServer_Run(t *testing.T) {
	s := NewServer("test", 1000)
	s.AddService("testservice", &TestService{},
		WithTimeout(1000),
		WithSerialization("json"))

	s.RunTcp(":8443")
}
