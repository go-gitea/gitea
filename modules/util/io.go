// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"io"
)

// FillBuffer reads exactly len(buf) bytes from r into buf.
// It returns the number of bytes copied. n is only less then len(buf) if r has fewer bytes.
// If EOF occurs while reading err will be nil.
func FillBuffer(r io.Reader, buf []byte) (n int, err error) {
	n, err = io.ReadFull(r, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return
}
