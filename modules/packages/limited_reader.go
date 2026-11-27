// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"archive/tar"
	"compress/gzip"
	"io"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var ErrContentTooLarge = util.NewInvalidArgumentErrorf("content exceeds the size limit")

const MaxMetadataSize = 32 * 1024 * 1024

// ArchiveDecompressionFloor is a var so tests can lower it
var ArchiveDecompressionFloor int64 = 512 * 1024 * 1024

// SizedReader is an uploaded buffer that knows its own compressed size
type SizedReader interface {
	io.Reader
	io.ReaderAt
	Size() int64
}

// NewArchiveLimitedReader bounds inflation while locating metadata, scaled to the compressed size so large packages still pass
func NewArchiveLimitedReader(r io.Reader, archiveSize int64) io.Reader {
	if setting.Packages.MaxDecompressionRatio < 0 {
		return r
	}
	return NewLimitedReader(r, max(ArchiveDecompressionFloor, setting.Packages.MaxDecompressionRatio*archiveSize))
}

// NewArchiveTarGzReader opens a .tar.gz upload with its inflation bounded by NewArchiveLimitedReader
func NewArchiveTarGzReader(r io.Reader, archiveSize int64) (*tar.Reader, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(NewArchiveLimitedReader(gzr, archiveSize)), nil
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
