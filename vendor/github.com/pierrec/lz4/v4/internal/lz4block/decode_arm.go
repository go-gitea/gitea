// +build gc,!noasm

package lz4block

func decodeBlock(dst, src, dict []byte) int {
	if len(dict) == 0 {
		return decodeBlockNodict(dst, src)
	}
	return decodeBlockGo(dst, src, dict)
}

// Assembler version of decodeBlock, without linked block support.

//go:noescape
func decodeBlockNodict(dst, src []byte) int
