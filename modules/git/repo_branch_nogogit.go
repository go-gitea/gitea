// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// IsObjectExist returns true if the given object exists in the repository.
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatchCheck(repo.Ctx)
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	defer cancel()
	_, err = batch.Writer().Write([]byte(name + "\n"))
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	sha, _, _, err := ReadBatchLine(batch.Reader())
	return err == nil && bytes.HasPrefix(sha, []byte(strings.TrimSpace(name)))
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatchCheck(repo.Ctx)
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	defer cancel()
	_, err = batch.Writer().Write([]byte(name + "\n"))
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	_, _, _, err = ReadBatchLine(batch.Reader())
	return err == nil
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if repo == nil || name == "" {
		return false
	}

	return repo.IsReferenceExist(BranchPrefix + name)
}
