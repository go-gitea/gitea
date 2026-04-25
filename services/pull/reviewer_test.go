// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestRepoGetReviewers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})

	// test public repo
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := t.Context()
	reviewers, err := pull_service.GetReviewers(ctx, repo1, user2, 0)
	assert.NoError(t, err)
	if assert.Len(t, reviewers, 1) {
		assert.ElementsMatch(t, []int64{2}, []int64{reviewers[0].ID})
	}

	// should not include doer and remove the poster
	reviewers, err = pull_service.GetReviewers(ctx, repo1, user11, 2)
	assert.NoError(t, err)
	assert.Empty(t, reviewers)

	// should not include PR poster, if PR poster would be otherwise eligible
	reviewers, err = pull_service.GetReviewers(ctx, repo1, user11, 4)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)

	// test private user repo
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	reviewers, err = pull_service.GetReviewers(ctx, repo2, user2, 4)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)
	assert.EqualValues(t, 2, reviewers[0].ID)

	// test private org repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	reviewers, err = pull_service.GetReviewers(ctx, repo3, user2, 1)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 2)

	reviewers, err = pull_service.GetReviewers(ctx, repo3, user2, 2)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 1)
}

// TestRepoGetReviewersHidesPrivateAndRestricted is a regression guard for
// issue #37371: GetReviewers must not expose users with visibility=private
// or is_restricted=true to non-admin doers who do not share an org with
// them. Repo4 is owned by user5 (non-org) and user29 (is_restricted=true)
// is already a collaborator there via fixtures.
func TestRepoGetReviewersHidesPrivateAndRestricted(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo4 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	// Non-admin owner: user29 (restricted collaborator) must be filtered out.
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	assert.False(t, user5.IsAdmin)

	reviewers, err := pull_service.GetReviewers(t.Context(), repo4, user5, 0)
	assert.NoError(t, err)
	for _, u := range reviewers {
		assert.NotEqual(t, int64(29), u.ID, "restricted user must not be returned to non-admin doer")
	}

	// Admin doer must still see the restricted collaborator.
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.True(t, admin.IsAdmin)

	reviewersAdmin, err := pull_service.GetReviewers(t.Context(), repo4, admin, 0)
	assert.NoError(t, err)
	seen := make(map[int64]bool, len(reviewersAdmin))
	for _, u := range reviewersAdmin {
		seen[u.ID] = true
	}
	assert.True(t, seen[29], "admin must see restricted collaborator")
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
