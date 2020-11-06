// +build amd64 arm
// +build !appengine
// +build gc
// +build !noasm

package lz4block

//go:noescape
func decodeBlock(dst, src []byte) int
