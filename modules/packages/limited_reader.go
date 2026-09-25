// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"

	"gitea.dev/modules/util"
)

// ErrContentTooLarge is returned when content read from an upload exceeds its size limit
var ErrContentTooLarge = util.NewInvalidArgumentErrorf("content exceeds the size limit")

const MaxMetadataSize = 32 * 1024 * 1024

type limitedReader struct {
	io.LimitedReader
}

// NewLimitedReader returns a reader that fails with ErrContentTooLarge once it reads a byte past limit
func NewLimitedReader(r io.Reader, limit int64) io.Reader {
	return &limitedReader{io.LimitedReader{R: r, N: limit + 1}}
}

func (l *limitedReader) Read(p []byte) (int, error) {
	n, err := l.LimitedReader.Read(p)
	if l.N == 0 {
		return max(n-1, 0), ErrContentTooLarge
	}
	return n, err
}
