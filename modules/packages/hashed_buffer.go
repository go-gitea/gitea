// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

// NewHashedBuffer creates a hashed buffer with a specific maximum memory size
func NewHashedBuffer(maxMemorySize int) (*HashedBuffer, error) {
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

// CreateHashedBufferFromReader creates a hashed buffer and copies the provided reader data into it.
func CreateHashedBufferFromReader(r io.Reader, maxMemorySize int) (*HashedBuffer, error) {
	b, err := NewHashedBuffer(maxMemorySize)
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
