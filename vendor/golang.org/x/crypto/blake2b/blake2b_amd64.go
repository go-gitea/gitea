// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !go1.7,amd64,!gccgo,!appengine

package blake2b
import "code.gitea.io/gitea/traceinit"

import "golang.org/x/sys/cpu"

func init() {
traceinit.Trace("./vendor/golang.org/x/crypto/blake2b/blake2b_amd64.go")
	useSSE4 = cpu.X86.HasSSE41
}

//go:noescape
func hashBlocksSSE4(h *[8]uint64, c *[2]uint64, flag uint64, blocks []byte)

func hashBlocks(h *[8]uint64, c *[2]uint64, flag uint64, blocks []byte) {
	if useSSE4 {
		hashBlocksSSE4(h, c, flag, blocks)
	} else {
		hashBlocksGeneric(h, c, flag, blocks)
	}
}
