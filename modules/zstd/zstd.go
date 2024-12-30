// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package zstd provides a high-level API for reading and writing zstd-compressed data.
// It supports both regular and seekable zstd streams.
// It's not a new wheel, but a wrapper around the zstd and zstd-seekable-format-go packages.
package zstd

import (
	"errors"
	"io"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

type Writer zstd.Encoder

var _ io.WriteCloser = (*Writer)(nil)

// NewWriter returns a new zstd writer.
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

// NewReader returns a new zstd reader.
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

// NewSeekableWriter returns a zstd writer to compress data to seekable format.
// blockSize is an important parameter, it should be decided according to the actual business requirements.
// If it's too small, the compression ratio could be very bad, even no compression at all.
// If it's too large, it could cost more traffic when reading the data partially from underlying storage.
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

type SeekableReader struct {
	r seekable.Reader
	c func() error
}

var _ io.ReadSeekCloser = (*SeekableReader)(nil)

// NewSeekableReader returns a zstd reader to decompress data from seekable format.
func NewSeekableReader(r io.ReadSeeker, opts ...ReaderOption) (*SeekableReader, error) {
	zstdR, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}

	seekableR, err := seekable.NewReader(r, zstdR)
	if err != nil {
		return nil, err
	}

	ret := &SeekableReader{
		r: seekableR,
	}
	if closer, ok := r.(io.Closer); ok {
		ret.c = closer.Close
	}

	return ret, nil
}

func (r *SeekableReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *SeekableReader) Seek(offset int64, whence int) (int64, error) {
	return r.r.Seek(offset, whence)
}

func (r *SeekableReader) Close() error {
	return errors.Join(
		func() error {
			if r.c != nil {
				return r.c()
			}
			return nil
		}(),
		r.r.Close(),
	)
}
