// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package objectpool

import "io"

type BufferedReader interface {
	io.Reader
	Buffered() int
	Peek(n int) ([]byte, error)
	Discard(n int) (int, error)
	ReadString(sep byte) (string, error)
	ReadSlice(sep byte) ([]byte, error)
	ReadBytes(sep byte) ([]byte, error)
}
