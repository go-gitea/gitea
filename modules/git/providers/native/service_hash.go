// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"crypto/sha1"
	"hash"
	"io"
	"strconv"

	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.HashService) = HashService{}

// HashService represents a native HashService
type HashService struct{}

// ComputeBlobHash compute the hash for a given blob content
func (hs HashService) ComputeBlobHash(size int64, content io.Reader) (service.Hash, error) {
	return hs.ComputeHash(service.ObjectBlob, size, content)
}

// ComputeHash compute the hash for a given ObjectType and content
func (HashService) ComputeHash(t service.ObjectType, size int64, content io.Reader) (service.Hash, error) {
	h := NewHasher(t, size)
	_, err := io.Copy(h, content)
	if err != nil {
		return nil, err
	}
	return h.Sum(), nil
}

// MustHashFromString converts a string to a hash
func (HashService) MustHashFromString(sha string) service.Hash {
	return StringHash(sha)
}

// Hasher is a struct that will generate a SHA1
type Hasher struct {
	hash.Hash
}

// NewHasher takes an object type and size and creates a hasher to generate a SHA
func NewHasher(t service.ObjectType, size int64) Hasher {
	h := Hasher{sha1.New()}
	_, _ = h.Write(t.Bytes())
	_, _ = h.Write([]byte(" "))
	_, _ = h.Write([]byte(strconv.FormatInt(size, 10)))
	_, _ = h.Write([]byte{0})
	return h
}

// Sum generates a SHA1 for the provided hash
func (h Hasher) Sum() (sha1 SHA1) {
	copy(sha1[:], h.Hash.Sum(nil))
	return
}
