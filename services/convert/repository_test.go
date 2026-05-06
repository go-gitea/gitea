// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRepoIncludesLastPullSyncSuccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo.IsMirror = true
	require.NoError(t, repo_model.DeleteMirrorByRepoID(ctx, repo.ID))
	repo.LastPullSyncSuccessUnix = timeutil.TimeStamp(1714000000)
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_mirror", "last_pull_sync_success_unix"))

	mirror := &repo_model.Mirror{
		RepoID:   repo.ID,
		Interval: time.Hour,
	}
	require.NoError(t, db.Insert(ctx, mirror))

	mirror.UpdatedUnix = timeutil.TimeStamp(1715000000)
	mirror.NextUpdateUnix = timeutil.TimeStamp(1716000000)
	require.NoError(t, repo_model.UpdateMirror(ctx, mirror))

	apiRepo := ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeRead})
	require.NotNil(t, apiRepo)

	assert.Equal(t, mirror.UpdatedUnix.AsTime(), apiRepo.MirrorUpdated)
	assert.Equal(t, repo.LastPullSyncSuccessUnix.AsTime(), apiRepo.LastPullSyncSuccess)
	assert.NotEqual(t, apiRepo.MirrorUpdated, apiRepo.LastPullSyncSuccess)
}
