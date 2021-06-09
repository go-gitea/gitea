// +build !appengine
// +build gc
// +build !noasm

package lz4block

//go:noescape
func decodeBlock(dst, src, dict []byte) int
