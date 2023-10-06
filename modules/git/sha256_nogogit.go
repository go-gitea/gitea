// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"encoding/hex"
)

// SHA256 a git commit name
type SHA256 [32]byte

var _ Hash = SHA256{}

// String returns a string representation of the SHA
func (s SHA256) String() string {
	return hex.EncodeToString(s[:])
}

// IsZero returns whether this SHA1 is all zeroes
func (s SHA256) IsZero() bool {
	var empty SHA256
	return s == empty
}

func (s SHA256) HashType() HashType {
	return Sha256HashType{}
}
