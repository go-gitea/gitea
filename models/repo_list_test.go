// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchRepositoryByName(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// test search public repository on explore page
	repos, count, err := SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "repo_12",
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_12", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "test_repo",
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)

	// test search private repository on explore page
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "repo_13",
		Page:     1,
		PageSize: 10,
		Private:  true,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_13", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "test_repo",
		Page:     1,
		PageSize: 10,
		Private:  true,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, repos, 3)

	// Get all public repositories by name
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "big_test_",
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
	assert.Len(t, repos, 4)

	// Get all public + private repositories by name
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "big_test_",
		Page:     1,
		PageSize: 10,
		Private:  true,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
	assert.Len(t, repos, 8)

	// Get all public + private repositories by name with pagesize limit (first page)
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "big_test_",
		Page:     1,
		PageSize: 5,
		Private:  true,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
	assert.Len(t, repos, 5)

	// Get all public + private repositories by name with pagesize limit (second page)
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "big_test_",
		Page:     2,
		PageSize: 5,
		Private:  true,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
	assert.Len(t, repos, 3)

	// Get all public repositories of user
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:     1,
		PageSize: 10,
		OwnerID:  15,
		Searcher: &User{ID: 15},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)

	// Get all public + private repositories of user
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:     1,
		PageSize: 10,
		OwnerID:  15,
		Private:  true,
		Searcher: &User{ID: 15},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
	assert.Len(t, repos, 4)

	// Get all public (including collaborative) repositories of user
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:        1,
		PageSize:    10,
		OwnerID:     15,
		Collaborate: true,
		Searcher:    &User{ID: 15},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
	assert.Len(t, repos, 4)

	// Get all public + private (including collaborative) repositories of user
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:        1,
		PageSize:    10,
		OwnerID:     15,
		Private:     true,
		Collaborate: true,
		Searcher:    &User{ID: 15},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
	assert.Len(t, repos, 8)

	// Get all public repositories of organization
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:     1,
		PageSize: 10,
		OwnerID:  17,
		Searcher: &User{ID: 17},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Len(t, repos, 1)

	// Get all public + private repositories of organization
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Page:     1,
		PageSize: 10,
		OwnerID:  17,
		Private:  true,
		Searcher: &User{ID: 17},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)
}
