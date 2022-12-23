// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"github.com/stretchr/testify/assert"
)

func Test_AddPrimaryKeyToForeignReference(t *testing.T) {
	// ForeignReference represents external references
	type ForeignReference struct {
		// RepoID is the first column in all indices. now we only need 2 indices: (repo, local) and (repo, foreign, type)
		RepoID       int64  `xorm:"UNIQUE(repo_foreign_type) INDEX(repo_local)" `
		LocalIndex   int64  `xorm:"INDEX(repo_local)"` // the resource key inside Gitea, it can be IssueIndex, or some model ID.
		ForeignIndex string `xorm:"INDEX UNIQUE(repo_foreign_type)"`
		Type         string `xorm:"VARCHAR(16) INDEX UNIQUE(repo_foreign_type)"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(ForeignReference))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := AddPrimaryKeyToForeignReference(x); err != nil {
		assert.NoError(t, err)
		return
	}
}
