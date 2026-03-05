// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUserAllowedToUpdate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	setRepoAllowRebaseUpdate := func(t *testing.T, repoID int64, allow bool) {
		repoUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repoID, Type: unit.TypePullRequests})
		repoUnit.PullRequestsConfig().AllowRebaseUpdate = allow
		require.NoError(t, repo_model.UpdateRepoUnit(t.Context(), repoUnit))
	}

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	t.Run("RespectsProtectedBranch", func(t *testing.T) {
		pr2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		protectedBranch := &git_model.ProtectedBranch{
			RepoID:       pr2.HeadRepoID,
			RuleName:     pr2.HeadBranch,
			CanPush:      false,
			CanForcePush: false,
		}
		_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
		require.NoError(t, err)
		defer db.DeleteByBean(t.Context(), protectedBranch)

		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr2, user2)
		assert.NoError(t, err)
		assert.False(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})

	t.Run("DisallowRebaseWhenConfigDisabled", func(t *testing.T) {
		pr2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		setRepoAllowRebaseUpdate(t, pr2.BaseRepoID, false)
		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr2, user2)
		assert.NoError(t, err)
		assert.True(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})

	t.Run("ReadOnlyAccessDenied", func(t *testing.T) {
		pr2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		collaboration := &repo_model.Collaboration{
			RepoID: pr2.HeadRepoID,
			UserID: user4.ID,
			Mode:   perm.AccessModeRead,
		}
		require.NoError(t, db.Insert(t.Context(), collaboration))
		defer db.DeleteByBean(t.Context(), collaboration)

		require.NoError(t, pr2.LoadHeadRepo(t.Context()))
		assert.NoError(t, access_model.RecalculateUserAccess(t.Context(), pr2.HeadRepo, user4.ID))

		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr2, user4)
		assert.NoError(t, err)
		assert.False(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})

	t.Run("ProtectedBranchAllowsPushWithoutRebase", func(t *testing.T) {
		pr2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		protectedBranch := &git_model.ProtectedBranch{
			RepoID:       pr2.HeadRepoID,
			RuleName:     pr2.HeadBranch,
			CanPush:      true,
			CanForcePush: false,
		}
		_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
		require.NoError(t, err)
		defer db.DeleteByBean(t.Context(), protectedBranch)

		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr2, user2)
		assert.NoError(t, err)
		assert.True(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})

	pr3Poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

	t.Run("MaintainerEditRespectsPosterPermissions", func(t *testing.T) {
		pr3 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
		pr3.AllowMaintainerEdit = true
		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr3, pr3Poster)
		assert.NoError(t, err)
		assert.False(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})

	t.Run("MaintainerEditInheritsPosterPermissions", func(t *testing.T) {
		pr3 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
		pr3.AllowMaintainerEdit = true
		protectedBranch := &git_model.ProtectedBranch{
			RepoID:       pr3.HeadRepoID,
			RuleName:     pr3.HeadBranch,
			CanPush:      true,
			CanForcePush: true,
		}
		_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
		require.NoError(t, err)
		defer db.DeleteByBean(t.Context(), protectedBranch)

		collaboration := &repo_model.Collaboration{
			RepoID: pr3.HeadRepoID,
			UserID: pr3Poster.ID,
			Mode:   perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), collaboration))
		defer db.DeleteByBean(t.Context(), collaboration)

		require.NoError(t, pr3.LoadHeadRepo(t.Context()))
		assert.NoError(t, access_model.RecalculateUserAccess(t.Context(), pr3.HeadRepo, pr3Poster.ID))

		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr3, pr3Poster)
		assert.NoError(t, err)
		assert.True(t, pushAllowed)
		assert.True(t, rebaseAllowed)
	})

	t.Run("MaintainerEditInheritsPosterPermissionsRebaseDisabled", func(t *testing.T) {
		pr3 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
		pr3.AllowMaintainerEdit = true
		protectedBranch := &git_model.ProtectedBranch{
			RepoID:       pr3.HeadRepoID,
			RuleName:     pr3.HeadBranch,
			CanPush:      true,
			CanForcePush: true,
		}
		_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
		require.NoError(t, err)
		defer db.DeleteByBean(t.Context(), protectedBranch)

		collaboration := &repo_model.Collaboration{
			RepoID: pr3.HeadRepoID,
			UserID: pr3Poster.ID,
			Mode:   perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), collaboration))
		defer db.DeleteByBean(t.Context(), collaboration)

		require.NoError(t, pr3.LoadHeadRepo(t.Context()))
		assert.NoError(t, access_model.RecalculateUserAccess(t.Context(), pr3.HeadRepo, pr3Poster.ID))

		setRepoAllowRebaseUpdate(t, pr3.BaseRepoID, false)

		pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr3, pr3Poster)
		assert.NoError(t, err)
		assert.True(t, pushAllowed)
		assert.False(t, rebaseAllowed)
	})
}
