// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/badges"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryBadges(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	badge := &badges.Badge{Slug: "repo-test-label", Description: "Repository test label"}
	assert.NoError(t, badges.CreateBadge(t.Context(), badge))

	assert.NoError(t, repo_model.AddRepoBadge(t.Context(), repo, badge))
	err := repo_model.AddRepoBadge(t.Context(), repo, badge)
	assert.ErrorIs(t, err, util.ErrAlreadyExist)

	got, err := repo_model.GetRepoBadges(t.Context(), repo)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, badge.Slug, got[0].Slug)

	results, count, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{
		Actor:       repo.Owner,
		Private:     true,
		OwnerID:     repo.OwnerID,
		BadgeSlug:   badge.Slug,
		Collaborate: optional.Some(false),
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1))
	assert.NotEmpty(t, results)
	assert.Equal(t, repo.ID, results[0].ID)

	assert.NoError(t, repo_model.RemoveRepoBadge(t.Context(), repo, badge))
	got, err = repo_model.GetRepoBadges(t.Context(), repo)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestSearchRepositoriesByBadge(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	badge := &badges.Badge{Slug: "repo-search-label", Description: "Repository search label"}
	assert.NoError(t, badges.CreateBadge(t.Context(), badge))
	assert.NoError(t, repo_model.AddRepoBadge(t.Context(), repo, badge))

	filtered, filteredCount, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{
		AllPublic: true,
		BadgeSlug: badge.Slug,
		ListOptions: db.ListOptions{
			ListAll: true,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), filteredCount)
	assert.Len(t, filtered, 1)
	assert.Equal(t, repo.ID, filtered[0].ID)

	withoutFilter, withoutFilterCount, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{
		AllPublic:   true,
		ListOptions: db.ListOptions{ListAll: true},
	})
	assert.NoError(t, err)
	assert.Greater(t, withoutFilterCount, filteredCount)
	assert.Greater(t, len(withoutFilter), len(filtered))
}
