// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var ErrContentTooLarge = util.NewInvalidArgumentErrorf("content exceeds the size limit")

const MaxMetadataSize = 32 * 1024 * 1024

const archiveDecompressionFloor = 512 * 1024 * 1024

// SizedReader is an uploaded buffer that knows its own compressed size
type SizedReader interface {
	io.Reader
	Size() int64
}

// NewArchiveLimitedReader bounds inflation while locating metadata, scaled to the compressed size so large packages still pass
func NewArchiveLimitedReader(r io.Reader, archiveSize int64) io.Reader {
	return NewLimitedReader(r, max(archiveDecompressionFloor, setting.Packages.MaxDecompressionRatio*archiveSize))
}

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
