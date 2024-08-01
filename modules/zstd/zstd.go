// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package zstd

import (
	"io"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

type Writer struct {
	enc *zstd.Encoder

	skw seekable.Writer
	buf []byte
	n   int
}

var _ io.WriteCloser = (*Writer)(nil)

func NewWriter(w io.Writer, opts ...WriterOption) (*Writer, error) {
	enc, err := zstd.NewWriter(w, opts...)
	if err != nil {
		return nil, err
	}
	return &Writer{
		enc: enc,
	}, nil
}

func NewSeekableWriter(w io.Writer, blockSize int, opts ...WriterOption) (*Writer, error) {
	enc, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		return nil, err
	}

	skw, err := seekable.NewWriter(w, enc)
	if err != nil {
		return nil, err
	}

	return &Writer{
		enc: enc,
		skw: skw,
		buf: make([]byte, blockSize),
	}, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	if w.skw != nil {
		return w.enc.Write(p)
	}

	written := 0
	for len(p) > 0 {
		n := copy(w.buf[w.n:], p)
		w.n += n
		written += n
		p = p[n:]

		if w.n == len(w.buf) {
			if _, err := w.skw.Write(w.buf); err != nil {
				return written, err
			}
			w.n = 0
		}
	}
	return written, nil
}

func (w *Writer) Close() error {
	if w.skw != nil {
		if w.n > 0 {
			if _, err := w.skw.Write(w.buf[:w.n]); err != nil {
				return err
			}
		}
		if err := w.skw.Close(); err != nil {
			return err
		}
	}
	return w.enc.Close()
}

type Reader struct {
	dec *zstd.Decoder
	skr seekable.Reader
}

var _ io.ReadCloser = (*Reader)(nil)

func NewReader(r io.Reader, opts ...ReaderOption) (*Reader, error) {
	dec, err := zstd.NewReader(r, opts...)
	if err != nil {
		return nil, err
	}
	return &Reader{
		dec: dec,
	}, nil
}

func (r *Reader) Read(p []byte) (int, error) {
	return r.dec.Read(p)
}

func (r *Reader) Close() error {
	r.dec.Close() // no error returned
	return nil
}

func (r *Reader) SeekReader() (seekable.Reader, error) {
	return r.skr
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
