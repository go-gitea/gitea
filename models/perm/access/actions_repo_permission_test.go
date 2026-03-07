// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActionsUserRepoPermission(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Use fixtures for repos and users
	repo4 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}) // Public repo
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}) // Private repo
	owner1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	owner2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	actionsUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user_model.ActionsUserID})

	t.Run("SameRepo_Public", func(t *testing.T) {
		// Task 47 belongs to repo 4 (public)
		task47 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
		require.Equal(t, repo4.ID, task47.RepoID)

		perm, err := GetActionsUserRepoPermission(ctx, repo4, actionsUser, task47.ID)
		require.NoError(t, err)

		// By default, it should have Read access because it's public
		assert.Equal(t, perm_model.AccessModeRead, perm.AccessMode)
		assert.Equal(t, perm_model.AccessModeWrite, perm.UnitAccessMode(unit.TypeCode))
	})

	t.Run("SameRepo_Private", func(t *testing.T) {
		// Task 53 belongs to repo 2 (private)
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		require.Equal(t, repo2.ID, task53.RepoID)

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Private repo, no extra access
		assert.Equal(t, perm_model.AccessModeNone, perm.AccessMode)
		assert.Equal(t, perm_model.AccessModeWrite, perm.UnitAccessMode(unit.TypeCode))
	})

	t.Run("CrossRepo_Allowed_All", func(t *testing.T) {
		// Org 1 owns repo 4 (public). Task 48 is in repo 4.
		// We want to access repo 1 (private, also owned by Org 1) from repo 4.
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		require.Equal(t, owner1.ID, repo4.OwnerID)
		require.Equal(t, owner1.ID, repo1.OwnerID)

		task48 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 48})
		require.Equal(t, repo4.ID, task48.RepoID)

		// Set owner policy to All
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeAll,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner1.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo1, actionsUser, task48.ID)
		require.NoError(t, err)

		// Should have read access to the private repo because of "All" policy.
		// Note: repo_permission.go logic for cross-repo access results in Read-Only clamping.
		assert.True(t, perm.CanRead(unit.TypeCode))
		assert.False(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("CrossRepo_Denied_None", func(t *testing.T) {
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		task48 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 48})

		// Set owner policy to None
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeNone,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner1.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo1, actionsUser, task48.ID)
		require.NoError(t, err)

		// Should NOT have access to the private repo.
		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("CrossRepo_Allowed_Selected", func(t *testing.T) {
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		task48 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 48})

		// Set owner policy to Selected
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode:       repo_model.ActionsCrossRepoModeSelected,
			AllowedCrossRepoIDs: []int64{repo1.ID},
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner1.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo1, actionsUser, task48.ID)
		require.NoError(t, err)

		assert.True(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("ForkPR_NoCrossRepo", func(t *testing.T) {
		// Task 53 is in repo 2. Let's make it a fork PR task.
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		task53.IsForkPullRequest = true
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "is_fork_pull_request"))

		// Even if policy is "All", fork PR should not have cross-repo access to other private repos.
		repo5 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
		require.Equal(t, owner2.ID, repo2.OwnerID)

		repo5.OwnerID = owner2.ID
		repo5.IsPrivate = true
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo5, "owner_id", "is_private"))

		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeAll,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo5, actionsUser, task53.ID)
		require.NoError(t, err)

		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("Inheritance_And_Clamping", func(t *testing.T) {
		// Repo 2 (Private). Task 53.
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		task53.IsForkPullRequest = false
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "is_fork_pull_request"))

		// Owner policy: Restricted mode (Read-only Code)
		ownerCfg := &repo_model.ActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModeRestricted,
			MaxTokenPermissions: &repo_model.ActionsTokenPermissions{
				Code: perm_model.AccessModeRead,
			},
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, ownerCfg))

		// Repo policy: OverrideOwnerConfig = false (should inherit owner's restricted mode)
		repo2ActionsUnit := repo2.MustGetUnit(ctx, unit.TypeActions)
		repo2ActionsCfg := repo2ActionsUnit.ActionsConfig()
		repo2ActionsCfg.OverrideOwnerConfig = false
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo2ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only because of inherited owner restricted mode
		assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeCode))
		assert.False(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("RepoOverride_Clamping", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})

		// Owner policy: Permissive (Write access)
		ownerCfg := &repo_model.ActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, ownerCfg))

		// Repo policy: OverrideOwnerConfig = true, but MaxTokenPermissions = Read
		repo2ActionsUnit := repo2.MustGetUnit(ctx, unit.TypeActions)
		repo2ActionsCfg := repo2ActionsUnit.ActionsConfig()
		repo2ActionsCfg.OverrideOwnerConfig = true
		repo2ActionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModeRestricted
		repo2ActionsCfg.MaxTokenPermissions = &repo_model.ActionsTokenPermissions{
			Code: perm_model.AccessModeRead,
		}
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo2ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only because of repo-level restriction
		assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeCode))
		assert.False(t, perm.CanWrite(unit.TypeCode))
	})
}
