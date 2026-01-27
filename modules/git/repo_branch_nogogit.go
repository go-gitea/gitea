// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// IsObjectExist returns true if the given object exists in the repository.
// FIXME: this function doesn't seem right, it is only used by GarbageCollectLFSMetaObjectsForRepo
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		log.Debug("Error opening CatFileBatch %v", err)
		return false
	}
	defer cancel()
	info, err := batch.QueryInfo(name)
	if err != nil {
		log.Debug("Error checking object info %v", err)
		return false
	}
	return strings.HasPrefix(info.ID, name) // FIXME: this logic doesn't seem right, why "HasPrefix"
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		log.Error("Error opening CatFileBatch %v", err)
		return false
	}
	defer cancel()
	_, err = batch.QueryInfo(name)
	return err == nil
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if repo == nil || name == "" {
		return false
	}

	return repo.IsReferenceExist(BranchPrefix + name)
}
