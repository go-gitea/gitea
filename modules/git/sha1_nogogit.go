// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build nogogit

package git

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"regexp"
	"strconv"
	"strings"
)

// EmptySHA defines empty git SHA
const EmptySHA = "0000000000000000000000000000000000000000"

// EmptyTreeSHA is the SHA of an empty tree
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// SHAPattern can be used to determine if a string is an valid sha
var SHAPattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// SHA1 a git commit name
type SHA1 [20]byte

// String returns a string representation of the SHA
func (s SHA1) String() string {
	return hex.EncodeToString(s[:])
}

func (s SHA1) IsZero() bool {
	var empty SHA1
	return s == empty
}

// MustID always creates a new SHA1 from a [20]byte array with no validation of input.
func MustID(b []byte) SHA1 {
	var id SHA1
	copy(id[:], b)
	return id
}

// NewID creates a new SHA1 from a [20]byte array.
func NewID(b []byte) (SHA1, error) {
	if len(b) != 20 {
		return SHA1{}, fmt.Errorf("Length must be 20: %v", b)
	}
	return MustID(b), nil
}

// MustIDFromString always creates a new sha from a ID with no validation of input.
func MustIDFromString(s string) SHA1 {
	b, _ := hex.DecodeString(s)
	return MustID(b)
}

// NewIDFromString creates a new SHA1 from a ID string of length 40.
func NewIDFromString(s string) (SHA1, error) {
	var id SHA1
	s = strings.TrimSpace(s)
	if len(s) != 40 {
		return id, fmt.Errorf("Length must be 40: %s", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, err
	}
	return NewID(b)
}

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(content []byte) SHA1 {
	return ComputeHash(ObjectBlob, content)
}

// ComputeHash compute the hash for a given ObjectType and content
func ComputeHash(t ObjectType, content []byte) SHA1 {
	h := NewHasher(t, int64(len(content)))
	h.Write(content)
	return h.Sum()
}

// Hasher is a struct that will generate a SHA1
type Hasher struct {
	hash.Hash
}

// NewHasher takes an object type and size and creates a hasher to generate a SHA
func NewHasher(t ObjectType, size int64) Hasher {
	h := Hasher{sha1.New()}
	h.Write(t.Bytes())
	h.Write([]byte(" "))
	h.Write([]byte(strconv.FormatInt(size, 10)))
	h.Write([]byte{0})
	return h
}

// Sum generates a SHA1 for the provided hash
func (h Hasher) Sum() (sha1 SHA1) {
	copy(sha1[:], h.Hash.Sum(nil))
	return
}
