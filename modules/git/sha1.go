// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EmptySHA1 defines empty git SHA (undefined, non-existent)
const EmptySHA1 = "0000000000000000000000000000000000000000"

// EmptyTreeSHA1 is the SHA of an empty tree, the root of all git repositories
const EmptyTreeSHA1 = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// SHA1FullLength is the full length of a git SHA
const SHA1FullLength = 40

// SHA1Pattern can be used to determine if a string is an valid sha
var sha1Pattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// IsValidSHAPattern will check if the provided string matches the SHA1 or SHA256 Pattern
func IsValidSHAPattern(sha string) bool {
	return IsValidSHA1Pattern(sha) || IsValidSHA256Pattern(sha)
}

// IsValidSHA1Pattern will check if the provided string matches the SHA1 Pattern
func IsValidSHA1Pattern(sha string) bool {
	return sha1Pattern.MatchString(sha)
}

type Sha1HashType struct{}

var _ HashType = Sha1HashType{}

func (ht Sha1HashType) Empty() string {
	return EmptySHA1
}

func (ht Sha1HashType) EmptyTree() string {
	return EmptyTreeSHA1
}

func (ht Sha1HashType) FullLength() int {
	return SHA1FullLength
}

func (ht Sha1HashType) IsValid(sha string) bool {
	return IsValidSHA1Pattern(sha)
}

// NewHashFromBytes always creates a new SHA1 from a [20]byte array with no validation of input.
func (ht Sha1HashType) NewHashFromBytes(b []byte) Hash {
	var id SHA1
	copy(id[:], b)
	return id
}

// ComputeHash compute the hash for a given ObjectType and content
func (ht Sha1HashType) ComputeHash(t ObjectType, content []byte) Hash {
	h := sha1.New()
	_, _ = h.Write(t.Bytes())
	_, _ = h.Write([]byte(" "))
	_, _ = h.Write([]byte(strconv.FormatInt(int64(len(content)), 10)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(content)

	var sha1 SHA1
	copy(sha1[:], h.Sum(nil))
	return sha1
}

type ErrInvalidSHA struct {
	SHA string
}

func (err ErrInvalidSHA) Error() string {
	return fmt.Sprintf("invalid sha: %s", err.SHA)
}

func NewHashFromStringByType(ht HashType, s string) (Hash, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return ht.NewHashFromBytes(b), nil
}

func MustHashFromStringByType(ht HashType, s string) Hash {
	id, err := NewHashFromStringByType(ht, s)
	if err != nil {
		panic(err)
	}
	return id
}

// NewIDFromString creates a new SHA1 from a ID string of length 40.
func NewIDFromString(s string) (Hash, error) {
	s = strings.TrimSpace(s)
	switch len(s) {
	case Sha1HashType{}.FullLength():
		return NewHashFromStringByType(Sha1HashType{}, s)
	case Sha256HashType{}.FullLength():
		return NewHashFromStringByType(Sha256HashType{}, s)
	default:
		return nil, fmt.Errorf("Length must be 40 or 64: %s", s)
	}
}

func MustIDFromString(s string) Hash {
	id, err := NewIDFromString(s)
	if err != nil {
		panic(err)
	}
	return id
}
