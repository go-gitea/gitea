// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPRBaseBranchSelection(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})

	assert.Equal(t, repo.DefaultBranch, repo.GetDefaultPRBaseBranch(ctx))

	repo.DefaultPRBaseBranch = "branch2"
	assert.Equal(t, "branch2", repo.GetDefaultPRBaseBranch(ctx))

	err := repo.ValidateDefaultPRBaseBranch(ctx, "does-not-exist")
	assert.True(t, IsErrDefaultPRBaseBranchNotExist(err))
}
