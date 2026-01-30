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
)

func setRepoAllowRebaseUpdate(t *testing.T, repoID int64, allow bool) {
	t.Helper()

	repoUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repoID, Type: unit.TypePullRequests})
	repoUnit.PullRequestsConfig().AllowRebaseUpdate = allow
	assert.NoError(t, repo_model.UpdateRepoUnit(t.Context(), repoUnit))
}

func TestIsUserAllowedToUpdateRespectsProtectedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	protectedBranch := &git_model.ProtectedBranch{
		RepoID:       pr.HeadRepoID,
		RuleName:     pr.HeadBranch,
		CanPush:      false,
		CanForcePush: false,
	}
	_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
	assert.NoError(t, err)

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.False(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}

func TestIsUserAllowedToUpdateDisablesRebaseWhenConfigDisabled(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	setRepoAllowRebaseUpdate(t, pr.BaseRepoID, false)

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.True(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}

func TestIsUserAllowedToUpdateReadOnlyAccessDenied(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	assert.NoError(t, db.Insert(t.Context(), &repo_model.Collaboration{
		RepoID: pr.HeadRepoID,
		UserID: user.ID,
		Mode:   perm.AccessModeRead,
	}))
	assert.NoError(t, access_model.RecalculateUserAccess(t.Context(), pr.HeadRepo, user.ID))

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.False(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}

func TestIsUserAllowedToUpdateProtectedBranchAllowsPushWithoutRebase(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	protectedBranch := &git_model.ProtectedBranch{
		RepoID:       pr.HeadRepoID,
		RuleName:     pr.HeadBranch,
		CanPush:      true,
		CanForcePush: false,
	}
	_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
	assert.NoError(t, err)

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.True(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}

func TestIsUserAllowedToUpdateMaintainerEditRespectsPosterPermissions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	pr.AllowMaintainerEdit = true
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.Issue.LoadPoster(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.False(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}

func TestIsUserAllowedToUpdateMaintainerEditInheritsPosterPermissions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	pr.AllowMaintainerEdit = true
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.Issue.LoadPoster(t.Context()))

	protectedBranch := &git_model.ProtectedBranch{
		RepoID:       pr.HeadRepoID,
		RuleName:     pr.HeadBranch,
		CanPush:      true,
		CanForcePush: true,
	}
	_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
	assert.NoError(t, err)

	assert.NoError(t, db.Insert(t.Context(), &repo_model.Collaboration{
		RepoID: pr.HeadRepoID,
		UserID: pr.Issue.Poster.ID,
		Mode:   perm.AccessModeWrite,
	}))
	assert.NoError(t, access_model.RecalculateUserAccess(t.Context(), pr.HeadRepo, pr.Issue.Poster.ID))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.True(t, pushAllowed)
	assert.True(t, rebaseAllowed)
}
