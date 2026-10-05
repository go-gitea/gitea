// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/badges"
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
