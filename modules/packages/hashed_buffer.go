// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"io"

	"code.gitea.io/gitea/modules/util/filebuffer"
)

// HashedBuffer is buffer which calculates multiple checksums
type HashedBuffer struct {
	*filebuffer.FileBackedBuffer

	md5    hash.Hash
	sha1   hash.Hash
	sha256 hash.Hash
	sha512 hash.Hash

	combinedWriter io.Writer
}

// NewHashedBuffer creates a hashed buffer with a specific maximum memory size
func NewHashedBuffer(maxMemorySize int) (*HashedBuffer, error) {
	b, err := filebuffer.New(maxMemorySize)
	if err != nil {
		return nil, err
	}
	md5 := md5.New()
	sha1 := sha1.New()
	sha256 := sha256.New()
	sha512 := sha512.New()

	combinedWriter := io.MultiWriter(b, md5, sha1, sha256, sha512)

	return &HashedBuffer{
		b,
		md5,
		sha1,
		sha256,
		sha512,
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
	hashMD5 = b.md5.Sum(nil)
	hashSHA1 = b.sha1.Sum(nil)
	hashSHA256 = b.sha256.Sum(nil)
	hashSHA512 = b.sha512.Sum(nil)
	return
}
