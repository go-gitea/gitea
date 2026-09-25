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
	r io.Reader
	n int64
}

// NewLimitedReader returns a reader that fails with ErrContentTooLarge instead of reading past limit bytes
func NewLimitedReader(r io.Reader, limit int64) io.Reader {
	return &limitedReader{r: r, n: limit}
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n < 0 {
		return 0, ErrContentTooLarge
	}
	if int64(len(p)) > l.n {
		p = p[:l.n+1] // one byte past the limit tells an overflow apart from a stream ending exactly on it
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	if l.n < 0 {
		return n + int(l.n), ErrContentTooLarge
	}
	return n, err
}
