// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
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
	repo4 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})   // Public, Owner 5, has Actions unit
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // Public, Owner 2, has Actions unit
	repo15 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 15}) // Private, Owner 2, no Actions unit in fixtures
	owner2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	actionsUser := user_model.NewActionsUser()

	// Ensure repo15 has an Actions unit for testing configuration
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
		RepoID: repo15.ID,
		Type:   unit.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))

	t.Run("SameRepo_Public", func(t *testing.T) {
		task47 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
		require.Equal(t, repo4.ID, task47.RepoID)

		perm, err := GetActionsUserRepoPermission(ctx, repo4, actionsUser, task47.ID)
		require.NoError(t, err)

		// Public repo, bot should have Read access even if not collaborator
		assert.Equal(t, perm_model.AccessModeRead, perm.AccessMode)
		assert.True(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("SameRepo_Private", func(t *testing.T) {
		// Make repo15 private and use a task from User 2
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		// Move task to repo15
		task53.RepoID = repo15.ID
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "repo_id"))

		perm, err := GetActionsUserRepoPermission(ctx, repo15, actionsUser, task53.ID)
		require.NoError(t, err)

		// Private repo, bot has no base access, but gets Write from effective tokens perms (Permissive by default)
		assert.Equal(t, perm_model.AccessModeNone, perm.AccessMode)
		assert.True(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("CrossRepo_Allowed_All", func(t *testing.T) {
		// Task 53 is now in repo 15 (Private, Owner 2).
		// We want to access repo 1 (Public, Owner 2) from repo 15.
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		require.Equal(t, repo15.ID, task53.RepoID)
		require.Equal(t, owner2.ID, repo15.OwnerID)
		require.Equal(t, owner2.ID, repo1.OwnerID)

		// Set owner policy to All
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeAll,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo1, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should have read access to the repo because of "All" policy.
		// Note: repo1 is public, so it has read anyway, but this verifies the logic doesn't crash.
		assert.True(t, perm.CanRead(unit.TypeCode))
		assert.False(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("CrossRepo_Denied_None", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})

		// Use a private repository as the target to verify "None" policy
		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		require.Equal(t, owner2.ID, repo2.OwnerID)
		require.True(t, repo2.IsPrivate)

		// Set owner policy to None
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeNone,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should NOT have access to the private repo.
		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("ForkPR_NoCrossRepo", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		task53.IsForkPullRequest = true
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "is_fork_pull_request"))

		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

		// Policy is "All"
		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeAll,
		}
		require.NoError(t, actions_model.SetUserActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Fork PR never gets cross-repo access to other private repos
		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("Inheritance_And_Clamping", func(t *testing.T) {
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
		repo15ActionsUnit := repo15.MustGetUnit(ctx, unit.TypeActions)
		repo15ActionsCfg := repo15ActionsUnit.ActionsConfig()
		repo15ActionsCfg.OverrideOwnerConfig = false
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo15ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo15, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only
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

		// Repo policy: OverrideOwnerConfig = true, MaxTokenPermissions = Read
		repo15ActionsUnit := repo15.MustGetUnit(ctx, unit.TypeActions)
		repo15ActionsCfg := repo15ActionsUnit.ActionsConfig()
		repo15ActionsCfg.OverrideOwnerConfig = true
		repo15ActionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModeRestricted
		repo15ActionsCfg.MaxTokenPermissions = &repo_model.ActionsTokenPermissions{
			Code: perm_model.AccessModeRead,
		}
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo15ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo15, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only
		assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeCode))
	})
}
