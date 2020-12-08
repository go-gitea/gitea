// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"bytes"
	"io"
	"strings"
)

// For some of the common Readers, we wrap and extend them to satisfy the
// compress.BufferedReader interface to improve performance.

type buffer struct {
	*bytes.Buffer
}

type bytesReader struct {
	*bytes.Reader
	pos int64
	buf []byte
	arr [512]byte
}

type stringReader struct {
	*strings.Reader
	pos int64
	buf []byte
	arr [512]byte
}

func (r *buffer) Buffered() int {
	return r.Len()
}

func (r *buffer) Peek(n int) ([]byte, error) {
	b := r.Bytes()
	if len(b) < n {
		return b, io.EOF
	}
	return b[:n], nil
}

func (r *buffer) Discard(n int) (int, error) {
	b := r.Next(n)
	if len(b) < n {
		return len(b), io.EOF
	}
	return n, nil
}

func (r *bytesReader) Buffered() int {
	r.update()
	if r.Len() > len(r.buf) {
		return len(r.buf)
	}
	return r.Len()
}

func (r *bytesReader) Peek(n int) ([]byte, error) {
	if n > len(r.arr) {
		return nil, io.ErrShortBuffer
	}

	// Return sub-slice of local buffer if possible.
	r.update()
	if len(r.buf) >= n {
		return r.buf[:n], nil
	}

	// Fill entire local buffer, and return appropriate sub-slice.
	cnt, err := r.ReadAt(r.arr[:], r.pos)
	r.buf = r.arr[:cnt]
	if cnt < n {
		return r.arr[:cnt], err
	}
	return r.arr[:n], nil
}

func (r *bytesReader) Discard(n int) (int, error) {
	var err error
	if n > r.Len() {
		n, err = r.Len(), io.EOF
	}
	r.Seek(int64(n), io.SeekCurrent)
	return n, err
}

// update reslices the internal buffer to be consistent with the read offset.
func (r *bytesReader) update() {
	pos, _ := r.Seek(0, io.SeekCurrent)
	if off := pos - r.pos; off >= 0 && off < int64(len(r.buf)) {
		r.buf, r.pos = r.buf[off:], pos
	} else {
		r.buf, r.pos = nil, pos
	}
}

func (r *stringReader) Buffered() int {
	r.update()
	if r.Len() > len(r.buf) {
		return len(r.buf)
	}
	return r.Len()
}

func (r *stringReader) Peek(n int) ([]byte, error) {
	if n > len(r.arr) {
		return nil, io.ErrShortBuffer
	}

	// Return sub-slice of local buffer if possible.
	r.update()
	if len(r.buf) >= n {
		return r.buf[:n], nil
	}

	// Fill entire local buffer, and return appropriate sub-slice.
	cnt, err := r.ReadAt(r.arr[:], r.pos)
	r.buf = r.arr[:cnt]
	if cnt < n {
		return r.arr[:cnt], err
	}
	return r.arr[:n], nil
}

func (r *stringReader) Discard(n int) (int, error) {
	var err error
	if n > r.Len() {
		n, err = r.Len(), io.EOF
	}
	r.Seek(int64(n), io.SeekCurrent)
	return n, err
}

// update reslices the internal buffer to be consistent with the read offset.
func (r *stringReader) update() {
	pos, _ := r.Seek(0, io.SeekCurrent)
	if off := pos - r.pos; off >= 0 && off < int64(len(r.buf)) {
		r.buf, r.pos = r.buf[off:], pos
	} else {
		r.buf, r.pos = nil, pos
	}
}
