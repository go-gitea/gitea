// +build linux,386 linux,arm linux,mips linux,mipsle

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unix
import "code.gitea.io/gitea/traceinit"

func init() {
traceinit.Trace("./vendor/golang.org/x/sys/unix/fcntl_linux_32bit.go")
	// On 32-bit Linux systems, the fcntl syscall that matches Go's
	// Flock_t type is SYS_FCNTL64, not SYS_FCNTL.
	fcntl64Syscall = SYS_FCNTL64
}
