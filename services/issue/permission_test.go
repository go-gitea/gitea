// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanEditIssueOrPullMeta(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Cross-repo PR: issue 8 / pull 3, poster user11, base repo10 (user12), head repo11 (user13)
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 8})
	require.NoError(t, issue.LoadPullRequest(t.Context()))
	require.NoError(t, issue.LoadRepo(t.Context()))

	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})
	baseOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})
	headOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
	// user4 has no relation to base or head by default
	stranger := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	basePermFor := func(u *user_model.User) access_model.Permission {
		p, err := access_model.GetDoerRepoPermission(t.Context(), issue.Repo, u)
		require.NoError(t, err)
		return p
	}

	t.Run("Poster", func(t *testing.T) {
		ok, err := CanEditIssueOrPullMeta(t.Context(), poster, issue, basePermFor(poster))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("BaseOwner", func(t *testing.T) {
		ok, err := CanEditIssueOrPullMeta(t.Context(), baseOwner, issue, basePermFor(baseOwner))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("HeadOwner", func(t *testing.T) {
		// Head owner is not the poster and has no write on base PR unit, but
		// owns the head repo — should be allowed to edit title/body.
		ok, err := CanEditIssueOrPullMeta(t.Context(), headOwner, issue, basePermFor(headOwner))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Stranger", func(t *testing.T) {
		ok, err := CanEditIssueOrPullMeta(t.Context(), stranger, issue, basePermFor(stranger))
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("HeadCollaboratorWrite", func(t *testing.T) {
		// Grant write collaboration on head only
		collaboration := &repo_model.Collaboration{
			RepoID: issue.PullRequest.HeadRepoID,
			UserID: stranger.ID,
			Mode:   perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), collaboration))
		defer db.DeleteByBean(t.Context(), collaboration)

		require.NoError(t, issue.PullRequest.LoadHeadRepo(t.Context()))
		require.NoError(t, access_model.RecalculateUserAccess(t.Context(), issue.PullRequest.HeadRepo, stranger.ID))

		ok, err := CanEditIssueOrPullMeta(t.Context(), stranger, issue, basePermFor(stranger))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("HeadCollaboratorReadOnly", func(t *testing.T) {
		collaboration := &repo_model.Collaboration{
			RepoID: issue.PullRequest.HeadRepoID,
			UserID: stranger.ID,
			Mode:   perm.AccessModeRead,
		}
		require.NoError(t, db.Insert(t.Context(), collaboration))
		defer db.DeleteByBean(t.Context(), collaboration)

		require.NoError(t, issue.PullRequest.LoadHeadRepo(t.Context()))
		require.NoError(t, access_model.RecalculateUserAccess(t.Context(), issue.PullRequest.HeadRepo, stranger.ID))

		ok, err := CanEditIssueOrPullMeta(t.Context(), stranger, issue, basePermFor(stranger))
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("RegularIssueNoHeadBypass", func(t *testing.T) {
		// Non-PR issue: head-repo write must not apply
		plainIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		require.False(t, plainIssue.IsPull)
		require.NoError(t, plainIssue.LoadRepo(t.Context()))
		plainPerm, err := access_model.GetDoerRepoPermission(t.Context(), plainIssue.Repo, stranger)
		require.NoError(t, err)

		ok, err := CanEditIssueOrPullMeta(t.Context(), stranger, plainIssue, plainPerm)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("NilDoer", func(t *testing.T) {
		ok, err := CanEditIssueOrPullMeta(t.Context(), nil, issue, basePermFor(poster))
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
