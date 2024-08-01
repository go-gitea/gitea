// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package zstd

import (
	"io"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

type Writer zstd.Encoder

var _ io.WriteCloser = (*Writer)(nil)

func NewWriter(w io.Writer, opts ...WriterOption) (*Writer, error) {
	zstdW, err := zstd.NewWriter(w, opts...)
	if err != nil {
		return nil, err
	}
	return (*Writer)(zstdW), nil
}

func (w *Writer) Write(p []byte) (int, error) {
	return (*zstd.Encoder)(w).Write(p)
}

func (w *Writer) Close() error {
	return (*zstd.Encoder)(w).Close()
}

type Reader zstd.Decoder

var _ io.ReadCloser = (*Reader)(nil)

func NewReader(r io.Reader, opts ...ReaderOption) (*Reader, error) {
	zstdR, err := zstd.NewReader(r, opts...)
	if err != nil {
		return nil, err
	}
	return (*Reader)(zstdR), nil
}

func (r *Reader) Read(p []byte) (int, error) {
	return (*zstd.Decoder)(r).Read(p)
}

func (r *Reader) Close() error {
	(*zstd.Decoder)(r).Close() // no error returned
	return nil
}

type SeekableWriter struct {
	buf []byte
	n   int
	w   seekable.Writer
}

var _ io.WriteCloser = (*SeekableWriter)(nil)

func NewSeekableWriter(w io.Writer, blockSize int, opts ...WriterOption) (*SeekableWriter, error) {
	zstdW, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		return nil, err
	}

	seekableW, err := seekable.NewWriter(w, zstdW)
	if err != nil {
		return nil, err
	}

	return &SeekableWriter{
		buf: make([]byte, blockSize),
		w:   seekableW,
	}, nil
}

func (w *SeekableWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n := copy(w.buf[w.n:], p)
		w.n += n
		written += n
		p = p[n:]

		if w.n == len(w.buf) {
			if _, err := w.w.Write(w.buf); err != nil {
				return written, err
			}
			w.n = 0
		}
	}
	return written, nil
}

func (w *SeekableWriter) Close() error {
	if w.n > 0 {
		if _, err := w.w.Write(w.buf[:w.n]); err != nil {
			return err
		}
	}
	return w.w.Close()
}

type SeekableReader seekable.Reader

var _ io.ReadSeekCloser = (SeekableReader)(nil)

func NewSeekableReader(r io.ReadSeeker, opts ...ReaderOption) (SeekableReader, error) {
	zstdR, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}

	return seekable.NewReader(r, zstdR)
}
