// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTargetBranchSelection(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})

	assert.Equal(t, repo.DefaultBranch, repo.GetPullRequestTargetBranch(ctx))

	repo.Units = nil
	prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
	assert.NoError(t, err)
	prConfig := prUnit.PullRequestsConfig()
	prConfig.DefaultTargetBranch = "branch2"
	prUnit.Config = prConfig
	assert.NoError(t, UpdateRepoUnit(ctx, prUnit))
	repo.Units = nil
	assert.Equal(t, "branch2", repo.GetPullRequestTargetBranch(ctx))
}
