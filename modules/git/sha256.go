// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha256"
	"regexp"
	"strconv"
)

// EmptySHA256 defines empty git SHA (undefined, non-existent)
const EmptySHA256 = "0000000000000000000000000000000000000000"

// EmptyTreeSHA256 is the SHA of an empty tree, the root of all git repositories
const EmptyTreeSHA256 = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// SHA256FullLength is the full length of a git SHA
const SHA256FullLength = 64

// sha256Pattern can be used to determine if a string is an valid sha
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// IsValidSHA256Pattern will check if the provided string matches the SHA Pattern
func IsValidSHA256Pattern(sha string) bool {
	return sha256Pattern.MatchString(sha)
}

type Sha256HashType struct{}

var _ HashType = Sha256HashType{}

func (ht Sha256HashType) Empty() string {
	return EmptySHA256
}

func (ht Sha256HashType) EmptyTree() string {
	return EmptyTreeSHA256
}

func (ht Sha256HashType) FullLength() int {
	return SHA256FullLength
}

func (ht Sha256HashType) IsValid(sha string) bool {
	return IsValidSHA256Pattern(sha)
}

// NewHashFromBytes always creates a new SHA1 from a [32]byte array with no validation of input.
func (ht Sha256HashType) NewHashFromBytes(b []byte) Hash {
	var id SHA256
	copy(id[:], b)
	return id
}

// ComputeHash compute the hash for a given ObjectType and content
func (ht Sha256HashType) ComputeHash(t ObjectType, content []byte) Hash {
	h := sha256.New()
	_, _ = h.Write(t.Bytes())
	_, _ = h.Write([]byte(" "))
	_, _ = h.Write([]byte(strconv.FormatInt(int64(len(content)), 10)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(content)

	var sha256 SHA256
	copy(sha256[:], h.Sum(nil))
	return sha256
}
