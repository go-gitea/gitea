// +build !amd64,!arm appengine !gc noasm

package lz4block

func decodeBlock(dst, src, dict []byte) int {
	return decodeBlockGo(dst, src, dict)
}
