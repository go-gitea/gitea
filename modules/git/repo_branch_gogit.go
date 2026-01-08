// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
)

// IsObjectExist returns true if the given object exists in the repository.
// FIXME: Inconsistent behavior with nogogit edition
// Unlike the implementation of IsObjectExist in nogogit edition, it does not support short hashes here.
// For example, IsObjectExist("153f451") will return false, but it will return true in nogogit edition.
// To fix this, the solution could be adding support for short hashes in gogit edition if it's really needed.
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	_, err := repo.gogitRepo.Object(plumbing.AnyObject, plumbing.NewHash(name))
	return err == nil
}

// IsReferenceExist returns true if given reference exists in the repository.
// FIXME: Inconsistent behavior with nogogit edition
// Unlike the implementation of IsObjectExist in nogogit edition, it does not support blob hashes here.
// For example, IsObjectExist([existing_blob_hash]) will return false, but it will return true in nogogit edition.
// To fix this, the solution could be refusing to support blob hashes in nogogit edition since a blob hash is not a reference.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	_, err := repo.gogitRepo.ResolveRevision(plumbing.Revision(name))

	return err == nil
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	reference, err := repo.gogitRepo.Reference(plumbing.ReferenceName(BranchPrefix+name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}
