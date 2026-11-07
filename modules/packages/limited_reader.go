// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"

	"gitea.dev/modules/util"
)

var ErrContentTooLarge = util.NewInvalidArgumentErrorf("content exceeds the size limit")

const MaxMetadataSize = 32 * 1024 * 1024

type limitedReader struct {
	io.LimitedReader
}

// NewLimitedReader returns a reader that fails with ErrContentTooLarge once it reads a byte past limit
func NewLimitedReader(r io.Reader, limit int64) io.Reader {
	return &limitedReader{io.LimitedReader{R: r, N: limit + 1}}
}

// NewArchiveLimitedReader bounds archive inflation like cargo does, the 100x ratio is 10x above regular packages
func NewArchiveLimitedReader(r io.Reader, archiveSize int64) io.Reader {
	return NewLimitedReader(r, max(512*1024*1024, 100*archiveSize))
}

func (l *limitedReader) Read(p []byte) (int, error) {
	n, err := l.LimitedReader.Read(p)
	if l.N == 0 {
		return max(n-1, 0), ErrContentTooLarge
	}
	return n, err
}
