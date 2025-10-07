// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import "io"

// attachmentLimitedReader returns a Reader that reads from r
// but errors with ErrAttachmentSizeExceed after n bytes.
// The underlying implementation is a *attachmentReader.
func attachmentLimitedReader(r io.Reader, n int64) io.Reader { return &attachmentReader{r, n} }

// A attachmentReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns ErrAttachmentSizeExceed when N <= 0.
// Underlying errors are passed through.
type attachmentReader struct {
	R io.Reader // underlying reader
	N int64     // max bytes remaining
}

func (l *attachmentReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, &ErrAttachmentSizeExceed{MaxSize: l.N}
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}
