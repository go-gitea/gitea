// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit

package git

import (
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"strconv"
)

// SHA1 a git commit name
type SHA1 [20]byte

// String returns a string representation of the SHA
func (s SHA1) String() string {
	return hex.EncodeToString(s[:])
}

// IsZero returns whether this SHA1 is all zeroes
func (s SHA1) IsZero() bool {
	var empty SHA1
	return s == empty
}

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(content []byte) SHA1 {
	return ComputeHash(ObjectBlob, content)
}

// ComputeHash compute the hash for a given ObjectType and content
func ComputeHash(t ObjectType, content []byte) SHA1 {
	h := NewHasher(t, int64(len(content)))
	_, _ = h.Write(content)
	return h.Sum()
}

// Hasher is a struct that will generate a SHA1
type Hasher struct {
	hash.Hash
}

// NewHasher takes an object type and size and creates a hasher to generate a SHA
func NewHasher(t ObjectType, size int64) Hasher {
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
	return sha1
}
