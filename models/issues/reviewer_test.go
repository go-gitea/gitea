// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewerCandidates(t *testing.T) {
	t.Run("CollaboratorReadAccess", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4, Mode: perm.AccessModeRead}))
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 0, user4)
		require.NoError(t, err)
		assert.True(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 2, 0)
		require.NoError(t, err)
		assert.Contains(t, reviewerIDs(reviewers), int64(4))
	})

	t.Run("PRTeamMember", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 61}) // public org repo
		user38 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 38})

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 40, user38)
		require.NoError(t, err)
		assert.True(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 40, 40)
		require.NoError(t, err)
		assert.Contains(t, reviewerIDs(reviewers), int64(38))
	})

	t.Run("CodeOnlyTeamOnPublicOrgRepo", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 61})
		require.NoError(t, repo.LoadOwner(ctx))
		require.False(t, repo.IsPrivate)

		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		team := &organization.Team{OrgID: repo.OwnerID, LowerName: "code-only", Name: "code-only", AccessMode: perm.AccessModeNone}
		require.NoError(t, db.Insert(ctx, team))
		require.NoError(t, db.Insert(ctx, &organization.TeamUser{OrgID: repo.OwnerID, TeamID: team.ID, UID: user5.ID}))
		require.NoError(t, db.Insert(ctx, &organization.TeamRepo{OrgID: repo.OwnerID, TeamID: team.ID, RepoID: repo.ID}))
		require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: repo.OwnerID, TeamID: team.ID, Type: unit.TypeCode, AccessMode: perm.AccessModeRead}))

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 40, user5)
		require.NoError(t, err)
		assert.False(t, ok, "code-only team member must not be eligible on a public org repo")

		reviewers, err := issues_model.GetReviewers(ctx, repo, 40, 40)
		require.NoError(t, err)
		assert.NotContains(t, reviewerIDs(reviewers), int64(5))
	})

	t.Run("PublicEveryoneAccessOnly", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 61})
		user8 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 40, user8)
		require.NoError(t, err)
		assert.False(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 40, 40)
		require.NoError(t, err)
		assert.NotContains(t, reviewerIDs(reviewers), int64(8))
	})

	t.Run("SiteAdminNotOtherwiseEligible", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 61})
		admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		require.True(t, admin.IsAdmin)

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 40, admin)
		require.NoError(t, err)
		assert.False(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 40, 40)
		require.NoError(t, err)
		assert.NotContains(t, reviewerIDs(reviewers), int64(1))
	})

	t.Run("IndividualOwner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 0, owner)
		require.NoError(t, err)
		assert.True(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 4, 0)
		require.NoError(t, err)
		assert.Contains(t, reviewerIDs(reviewers), int64(2))
	})

	t.Run("PosterAndInactive", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo.ID, UserID: 9, Mode: perm.AccessModeRead}))
		inactive := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
		require.False(t, inactive.IsActive)

		ok, err := issues_model.IsUserReviewerCandidate(ctx, repo, 0, inactive)
		require.NoError(t, err)
		assert.False(t, ok)

		reviewers, err := issues_model.GetReviewers(ctx, repo, 2, 0)
		require.NoError(t, err)
		assert.NotContains(t, reviewerIDs(reviewers), int64(9))

		poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, db.Insert(ctx, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4, Mode: perm.AccessModeRead}))
		ok, err = issues_model.IsUserReviewerCandidate(ctx, repo, 4, poster)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("TeamEligibility", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 61})
		teams, err := issues_model.GetReviewerTeams(ctx, repo)
		require.NoError(t, err)
		require.NotEmpty(t, teams)

		ok, err := issues_model.IsTeamReviewerCandidate(ctx, repo, teams[0].ID)
		require.NoError(t, err)
		assert.True(t, ok)

		ok, err = issues_model.IsTeamReviewerCandidate(ctx, repo, 999999)
		require.NoError(t, err)
		assert.False(t, ok)

		userRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		teams, err = issues_model.GetReviewerTeams(ctx, userRepo)
		require.NoError(t, err)
		assert.Empty(t, teams)
	})
}

func reviewerIDs(users []*user_model.User) []int64 {
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}
