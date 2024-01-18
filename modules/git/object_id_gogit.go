// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/hash"
)

func ParseGogitHash(h plumbing.Hash) ObjectID {
	switch hash.Size {
	case 20:
		return Sha1ObjectFormat.MustID(h[:])
	}

	return nil
}

func ParseGogitHashArray(objectIDs []plumbing.Hash) []ObjectID {
	ret := make([]ObjectID, len(objectIDs))
	for i, h := range objectIDs {
		ret[i] = ParseGogitHash(h)
	}

	return ret
}
