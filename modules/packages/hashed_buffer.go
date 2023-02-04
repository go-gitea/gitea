// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"

	"code.gitea.io/gitea/modules/util/filebuffer"
)

// HashedSizeReader provide methods to read, sum hashes and a Size method
type HashedSizeReader interface {
	io.Reader
	HashSummer
	Size() int64
}

// HashedBuffer is buffer which calculates multiple checksums
type HashedBuffer struct {
	*filebuffer.FileBackedBuffer

	hash *MultiHasher

	combinedWriter io.Writer
}

const DefaultMemorySize = 32 * 1024 * 1024

// NewHashedBuffer creates a hashed buffer with the default memory size
func NewHashedBuffer() (*HashedBuffer, error) {
	return NewHashedBufferWithSize(DefaultMemorySize)
}

// NewHashedBuffer creates a hashed buffer with a specific memory size
func NewHashedBufferWithSize(maxMemorySize int) (*HashedBuffer, error) {
	b, err := filebuffer.New(maxMemorySize)
	if err != nil {
		return nil, err
	}

	hash := NewMultiHasher()

	combinedWriter := io.MultiWriter(b, hash)

	return &HashedBuffer{
		b,
		hash,
		combinedWriter,
	}, nil
}

// CreateHashedBufferFromReader creates a hashed buffer with the default memory size and copies the provided reader data into it.
func CreateHashedBufferFromReader(r io.Reader) (*HashedBuffer, error) {
	return CreateHashedBufferFromReaderWithSize(r, DefaultMemorySize)
}

// CreateHashedBufferFromReaderWithSize creates a hashed buffer and copies the provided reader data into it.
func CreateHashedBufferFromReaderWithSize(r io.Reader, maxMemorySize int) (*HashedBuffer, error) {
	b, err := NewHashedBufferWithSize(maxMemorySize)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(b, r)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Write implements io.Writer
func (b *HashedBuffer) Write(p []byte) (int, error) {
	return b.combinedWriter.Write(p)
}

// Sums gets the MD5, SHA1, SHA256 and SHA512 checksums of the data
func (b *HashedBuffer) Sums() (hashMD5, hashSHA1, hashSHA256, hashSHA512 []byte) {
	return b.hash.Sums()
}
