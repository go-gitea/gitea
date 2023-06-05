// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"io"
)

// BreakWriter wraps an io.Writer to always write '\n' as '<br>'
type BreakWriter struct {
	io.Writer
}

// Write writes the provided byte slice transparently replacing '\n' with '<br>'
func (b *BreakWriter) Write(bs []byte) (n int, err error) {
	pos := 0
	for pos < len(bs) {
		idx := bytes.IndexByte(bs[pos:], '\n')
		if idx < 0 {
			wn, err := b.Writer.Write(bs[pos:])
			return n + wn, err
		}

		if idx > 0 {
			wn, err := b.Writer.Write(bs[pos : pos+idx])
			n += wn
			if err != nil {
				return n, err
			}
		}

		if _, err = b.Writer.Write([]byte("<br>")); err != nil {
			return n, err
		}
		pos += idx + 1

		n++
	}

	return n, err
}
