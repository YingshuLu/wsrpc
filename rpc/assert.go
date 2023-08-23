// Package rpc
package rpc

func Assert(should bool, message string) {
	if !should {
		panic(message)
	}
}
