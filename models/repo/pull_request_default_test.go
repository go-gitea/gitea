// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.NoError(t, UpdateRepoUnitConfig(ctx, prUnit))
	repo.Units = nil
	assert.Equal(t, "branch2", repo.GetPullRequestTargetBranch(ctx))
}

func TestDefaultBaseRepoSelection(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	forkRepo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 11})
	baseRepo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 10})
	require.NoError(t, db.Insert(ctx, &RepoUnit{RepoID: forkRepo.ID, Type: unit.TypePullRequests, Config: DefaultPullRequestsConfig()}))
	forkRepo.Units = nil

	defaultBaseRepo, err := forkRepo.GetPullRequestDefaultBaseRepo(ctx)
	require.NoError(t, err)
	require.NotNil(t, defaultBaseRepo)
	assert.Equal(t, baseRepo.ID, defaultBaseRepo.ID)

	require.NoError(t, SetArchiveRepoState(ctx, baseRepo, true))
	t.Cleanup(func() {
		assert.NoError(t, SetArchiveRepoState(context.Background(), baseRepo, false))
	})

	forkRepo.BaseRepo = nil
	defaultBaseRepo, err = forkRepo.GetPullRequestDefaultBaseRepo(ctx)
	require.NoError(t, err)
	require.NotNil(t, defaultBaseRepo)
	assert.Equal(t, forkRepo.ID, defaultBaseRepo.ID)
}
