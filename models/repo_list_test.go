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
		Searcher: nil,
	})

	assert.NotNil(t, repos)
	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_12", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "test_repo",
		Page:     1,
		PageSize: 10,
		Searcher: nil,
	})

	assert.NotNil(t, repos)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// test search private repository on explore page
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:  "repo_13",
		Page:     1,
		PageSize: 10,
		Private:  true,
		Searcher: &User{ID: 14},
	})

	assert.NotNil(t, repos)
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
		Searcher: &User{ID: 14},
	})

	assert.NotNil(t, repos)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
