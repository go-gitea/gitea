// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
)

// SHA1 a git commit name
type SHA1 = plumbing.Hash

// ComputeBlobHash compute the hash for a given blob content
func ComputeBlobHash(content []byte) SHA1 {
	return plumbing.ComputeHash(plumbing.BlobObject, content)
}
