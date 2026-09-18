// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"

	"gitea.dev/modules/util"
)

// ErrPackageTooLarge is returned when an uploaded package decompresses past the budget its parser allows
var ErrPackageTooLarge = util.NewInvalidArgumentErrorf("package exceeds the decompression size limit")

// MaxMetadataScanSize bounds the decompressed bytes
const MaxMetadataScanSize = 32 << 20

type limitedDecompressor struct {
	r io.Reader
	n int64
}

// NewLimitedDecompressor returns a reader that fails with ErrPackageTooLarge instead of reporting EOF
func NewLimitedDecompressor(r io.Reader, budget int64) io.Reader {
	return &limitedDecompressor{r: r, n: budget}
}

func (l *limitedDecompressor) Read(p []byte) (int, error) {
	if int64(len(p)) > l.n+1 {
		p = p[:l.n+1] // one byte past the budget tells an overflow apart from a stream ending exactly on it
	}
	n, err := l.r.Read(p)
	if int64(n) > l.n {
		return int(l.n), ErrPackageTooLarge
	}
	l.n -= int64(n)
	return n, err
}
