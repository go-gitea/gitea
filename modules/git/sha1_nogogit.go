// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"encoding/hex"
)

// SHA1 a git commit name
type SHA1 [20]byte

var _ Hash = SHA1{}

// String returns a string representation of the SHA
func (s SHA1) String() string {
	return hex.EncodeToString(s[:])
}

func (s SHA1) Bytes() []byte {
	return s[:]
}

// IsZero returns whether this SHA1 is all zeroes
func (s SHA1) IsZero() bool {
	var empty SHA1
	return s == empty
}

func (s SHA1) HashType() HashType {
	return Sha1HashType{}
}

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(ht HashType, content []byte) Hash {
	return ht.ComputeHash(ObjectBlob, content)
}
