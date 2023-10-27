// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
)

// SHA1 a git commit name
type SHA1 struct {
	plumbing.Hash
}

func (h SHA1) HashType() HashType {
	return Sha1HashType{}
}

func (h SHA1) Bytes() []byte {
	return h.Hash[:]
}

type SHA256 struct {
	plumbing.Hash
}

func (h SHA256) HashType() HashType {
	return Sha256HashType{}
}

func (h SHA256) Bytes() []byte {
	return h.Hash[:]
}

var (
	_ Hash = SHA1{}
	_ Hash = SHA256{}
)

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(ht HashType, content []byte) Hash {
	if ht.FullLength() == SHA256FullLength {
		return SHA256{plumbing.ComputeHash(plumbing.BlobObject, content)}
	}
	return SHA1{plumbing.ComputeHash(plumbing.BlobObject, content)}
}
