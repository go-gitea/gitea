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
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})   // Private, Owner 2, no Actions unit in fixtures
	repo15 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 15}) // Private, Owner 2, no Actions unit in fixtures
	owner2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	actionsUser := user_model.NewActionsUser()

	// Ensure repo2 and repo15 have Actions units for testing configuration
	for _, r := range []*repo_model.Repository{repo2, repo15} {
		require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
			RepoID: r.ID,
			Type:   unit.TypeActions,
			Config: &repo_model.ActionsConfig{},
		}))
	}

	t.Run("SameRepo_Public", func(t *testing.T) {
		task47 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
		require.Equal(t, repo4.ID, task47.RepoID)

		perm, err := GetActionsUserRepoPermission(ctx, repo4, actionsUser, task47.ID)
		require.NoError(t, err)

		// Public repo, bot should have Read access even if not collaborator
		assert.Equal(t, perm_model.AccessModeNone, perm.AccessMode)
		assert.True(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("SameRepo_Private", func(t *testing.T) {
		// Use Task 53 which is already in Repo 2 (Private)
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		require.Equal(t, repo2.ID, task53.RepoID)

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Private repo, bot has no base access, but gets Write from effective tokens perms (Permissive by default)
		assert.Equal(t, perm_model.AccessModeNone, perm.AccessMode)
		assert.True(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("CrossRepo_Denied_None", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})

		// Set owner policy to nil allowed repos (None)
		cfg := actions_model.OwnerActionsConfig{}
		require.NoError(t, actions_model.SetOwnerActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo15, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should NOT have access to the private repo.
		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("ForkPR_NoCrossRepo", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		task53.IsForkPullRequest = true
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "is_fork_pull_request"))

		// Policy contains repo15
		cfg := actions_model.OwnerActionsConfig{
			AllowedCrossRepoIDs: []int64{repo15.ID},
		}
		require.NoError(t, actions_model.SetOwnerActionsConfig(ctx, owner2.ID, cfg))

		perm, err := GetActionsUserRepoPermission(ctx, repo15, actionsUser, task53.ID)
		require.NoError(t, err)

		// Fork PR never gets cross-repo access to other private repos
		assert.False(t, perm.CanRead(unit.TypeCode))
	})

	t.Run("Inheritance_And_Clamping", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
		task53.IsForkPullRequest = false
		require.NoError(t, actions_model.UpdateTask(ctx, task53, "is_fork_pull_request"))

		// Owner policy: Restricted mode (Read-only Code)
		ownerCfg := actions_model.OwnerActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModeRestricted,
			MaxTokenPermissions: &repo_model.ActionsTokenPermissions{
				UnitAccessModes: map[unit.Type]perm_model.AccessMode{
					unit.TypeCode: perm_model.AccessModeRead,
				},
			},
		}
		require.NoError(t, actions_model.SetOwnerActionsConfig(ctx, owner2.ID, ownerCfg))

		// Repo policy: OverrideOwnerConfig = false (should inherit owner's restricted mode)
		repo2ActionsUnit := repo2.MustGetUnit(ctx, unit.TypeActions)
		repo2ActionsCfg := repo2ActionsUnit.ActionsConfig()
		repo2ActionsCfg.OverrideOwnerConfig = false
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo2ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only
		assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeCode))
		assert.False(t, perm.CanWrite(unit.TypeCode))
	})

	t.Run("RepoOverride_Clamping", func(t *testing.T) {
		task53 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})

		// Owner policy: Permissive (Write access)
		ownerCfg := actions_model.OwnerActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
		}
		require.NoError(t, actions_model.SetOwnerActionsConfig(ctx, owner2.ID, ownerCfg))

		// Repo policy: OverrideOwnerConfig = true, MaxTokenPermissions = Read
		repo2ActionsUnit := repo2.MustGetUnit(ctx, unit.TypeActions)
		repo2ActionsCfg := repo2ActionsUnit.ActionsConfig()
		repo2ActionsCfg.OverrideOwnerConfig = true
		repo2ActionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModeRestricted
		repo2ActionsCfg.MaxTokenPermissions = &repo_model.ActionsTokenPermissions{
			UnitAccessModes: map[unit.Type]perm_model.AccessMode{
				unit.TypeCode: perm_model.AccessModeRead,
			},
		}
		require.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, repo2ActionsUnit))

		perm, err := GetActionsUserRepoPermission(ctx, repo2, actionsUser, task53.ID)
		require.NoError(t, err)

		// Should be clamped to Read-only
		assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeCode))
	})
}
