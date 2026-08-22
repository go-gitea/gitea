// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeTaskTokenPermissionsIDToken(t *testing.T) {
	writePermissions := func() *repo_model.ActionsTokenPermissions {
		permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
		permissions.IDTokenAccessMode = perm.AccessModeWrite
		return &permissions
	}
	loadTask := func(t *testing.T) (*ActionTask, *repo_model.Repository) {
		require.NoError(t, unittest.PrepareTestDatabase())
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{
			RepoID: repo.ID,
			Type:   unit.TypeActions,
			Config: &repo_model.ActionsConfig{},
		}))
		task := &ActionTask{RepoID: repo.ID, Job: &ActionRunJob{
			RunID:            990001,
			RepoID:           repo.ID,
			OwnerID:          repo.OwnerID,
			Repo:             repo,
			TokenPermissions: writePermissions(),
		}}
		return task, repo
	}

	t.Run("explicit write", func(t *testing.T) {
		task, repo := loadTask(t)
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeWrite, effective.IDTokenAccessMode)
	})

	for name, mode := range map[string]perm.AccessMode{
		"none": perm.AccessModeNone,
		"read": perm.AccessModeRead,
	} {
		t.Run(name, func(t *testing.T) {
			task, repo := loadTask(t)
			task.Job.TokenPermissions.IDTokenAccessMode = mode
			effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
			require.NoError(t, err)
			assert.Equal(t, mode, effective.IDTokenAccessMode)
		})
	}

	t.Run("omitted", func(t *testing.T) {
		task, repo := loadTask(t)
		task.Job.TokenPermissions = nil
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("owner maximum", func(t *testing.T) {
		task, repo := loadTask(t)
		maximum := repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
		maximum.IDTokenAccessMode = perm.AccessModeNone
		require.NoError(t, SetOwnerActionsConfig(t.Context(), repo.OwnerID, OwnerActionsConfig{MaxTokenPermissions: &maximum}))
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("repository maximum", func(t *testing.T) {
		task, repo := loadTask(t)
		actionsUnit := repo.MustGetUnit(t.Context(), unit.TypeActions)
		actionsUnit.ActionsConfig().OverrideOwnerConfig = true
		maximum := repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
		maximum.IDTokenAccessMode = perm.AccessModeNone
		actionsUnit.ActionsConfig().MaxTokenPermissions = &maximum
		require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), actionsUnit))
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("fork pull request", func(t *testing.T) {
		task, repo := loadTask(t)
		task.IsForkPullRequest = true
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("cross repository", func(t *testing.T) {
		task, _ := loadTask(t)
		targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		effective, err := ComputeTaskTokenPermissions(t.Context(), task, targetRepo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("explicit reusable caller restricts child", func(t *testing.T) {
		task, repo := loadTask(t)
		callerPermissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
		caller := &ActionRunJob{
			RunID:            task.Job.RunID,
			RepoID:           task.Job.RepoID,
			OwnerID:          task.Job.OwnerID,
			TokenPermissions: &callerPermissions,
			IsReusableCaller: true,
		}
		require.NoError(t, db.Insert(t.Context(), caller))
		task.Job.ParentJobID = caller.ID

		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("omitted root reusable caller uses defaults", func(t *testing.T) {
		task, repo := loadTask(t)
		caller := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), caller))
		task.Job.ParentJobID = caller.ID

		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("omitted nested caller remains transparent", func(t *testing.T) {
		task, repo := loadTask(t)
		rootPermissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
		root := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, TokenPermissions: &rootPermissions, IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), root))
		middle := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, ParentJobID: root.ID, IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), middle))
		task.Job.ParentJobID = middle.ID

		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})

	t.Run("omitted intermediate reusable caller is transparent", func(t *testing.T) {
		task, repo := loadTask(t)
		root := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, TokenPermissions: writePermissions(), IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), root))
		middle := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, ParentJobID: root.ID, IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), middle))
		task.Job.ParentJobID = middle.ID

		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeWrite, effective.IDTokenAccessMode)
	})

	t.Run("omitted reusable child uses defaults", func(t *testing.T) {
		task, repo := loadTask(t)
		caller := &ActionRunJob{RunID: task.Job.RunID, RepoID: task.Job.RepoID, OwnerID: task.Job.OwnerID, TokenPermissions: writePermissions(), IsReusableCaller: true}
		require.NoError(t, db.Insert(t.Context(), caller))
		task.Job.ParentJobID = caller.ID
		task.Job.TokenPermissions = nil

		effective, err := ComputeTaskTokenPermissions(t.Context(), task, repo)
		require.NoError(t, err)
		assert.Equal(t, perm.AccessModeNone, effective.IDTokenAccessMode)
	})
}
