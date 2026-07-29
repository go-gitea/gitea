// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	pull_service "gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoGetReviewers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// test public repo
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := t.Context()
	reviewers, err := pull_service.GetReviewers(ctx, repo1, 2, 0)
	assert.NoError(t, err)
	if assert.Len(t, reviewers, 1) {
		assert.ElementsMatch(t, []int64{2}, []int64{reviewers[0].ID})
	}

	// should not include doer and remove the poster
	reviewers, err = pull_service.GetReviewers(ctx, repo1, 11, 2)
	assert.NoError(t, err)
	assert.Empty(t, reviewers)

	// should not include PR poster, if PR poster would be otherwise eligible
	reviewers, err = pull_service.GetReviewers(ctx, repo1, 11, 4)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)

	// test private user repo
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	reviewers, err = pull_service.GetReviewers(ctx, repo2, 2, 4)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)
	assert.EqualValues(t, 2, reviewers[0].ID)

	// test private org repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	reviewers, err = pull_service.GetReviewers(ctx, repo3, 2, 1)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 2)

	reviewers, err = pull_service.GetReviewers(ctx, repo3, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)
}

func TestRepoGetReviewersPrivateVisibility(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31})
	collaboration := &repo_model.Collaboration{
		RepoID: repo.ID,
		UserID: privateUser.ID,
		Mode:   perm.AccessModeRead,
	}
	require.NoError(t, db.Insert(ctx, collaboration))
	defer db.DeleteByBean(ctx, collaboration)

	t.Run("UnrelatedDoer", func(t *testing.T) {
		reviewers, err := pull_service.GetReviewers(ctx, repo, 4, 0)
		require.NoError(t, err)
		require.Len(t, reviewers, 1)
		assert.EqualValues(t, 2, reviewers[0].ID)
	})

	t.Run("DoerInSameOrganization", func(t *testing.T) {
		reviewers, err := pull_service.GetReviewers(ctx, repo, 20, 0)
		require.NoError(t, err)

		reviewerIDs := make([]int64, 0, len(reviewers))
		for _, reviewer := range reviewers {
			reviewerIDs = append(reviewerIDs, reviewer.ID)
		}
		assert.ElementsMatch(t, []int64{2, 31}, reviewerIDs)
	})
}

func TestRepoGetReviewerTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	teams, err := pull_service.GetReviewerTeams(t.Context(), repo2)
	assert.NoError(t, err)
	assert.Empty(t, teams)

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	teams, err = pull_service.GetReviewerTeams(t.Context(), repo3)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}
