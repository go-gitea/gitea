// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package compress is a collection of compression libraries.
package compress

import (
	"bufio"
	"io"

	"github.com/dsnet/compress/internal/errors"
)

// The Error interface identifies all compression related errors.
type Error interface {
	error
	CompressError()

	// IsDeprecated reports the use of a deprecated and unsupported feature.
	IsDeprecated() bool

	// IsCorrupted reports whether the input stream was corrupted.
	IsCorrupted() bool
}

var _ Error = errors.Error{}

// ByteReader is an interface accepted by all decompression Readers.
// It guarantees that the decompressor never reads more data than is necessary
// from the underlying io.Reader.
type ByteReader interface {
	io.Reader
	io.ByteReader
}

var _ ByteReader = (*bufio.Reader)(nil)

// BufferedReader is an interface accepted by all decompression Readers.
// It guarantees that the decompressor never reads more data than is necessary
// from the underlying io.Reader. Since BufferedReader allows a decompressor
// to peek at bytes further along in the stream without advancing the read
// pointer, decompression can experience a significant performance gain when
// provided a reader that satisfies this interface. Thus, a decompressor will
// prefer this interface over ByteReader for performance reasons.
//
// The bufio.Reader satisfies this interface.
type BufferedReader interface {
	io.Reader

	// Buffered returns the number of bytes currently buffered.
	//
	// This value becomes invalid following the next Read/Discard operation.
	Buffered() int

	// Peek returns the next n bytes without advancing the reader.
	//
	// If Peek returns fewer than n bytes, it also returns an error explaining
	// why the peek is short. Peek must support peeking of at least 8 bytes.
	// If 0 <= n <= Buffered(), Peek is guaranteed to succeed without reading
	// from the underlying io.Reader.
	//
	// This result becomes invalid following the next Read/Discard operation.
	Peek(n int) ([]byte, error)

	// Discard skips the next n bytes, returning the number of bytes discarded.
	//
	// If Discard skips fewer than n bytes, it also returns an error.
	// If 0 <= n <= Buffered(), Discard is guaranteed to succeed without reading
	// from the underlying io.Reader.
	Discard(n int) (int, error)
}

var _ BufferedReader = (*bufio.Reader)(nil)
