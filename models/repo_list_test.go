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
		Searcher: &User{ID: 14},
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
		Searcher: &User{ID: 14},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, repos, 3)

	testCases := []struct {
		name  string
		opts  *SearchRepoOptions
		count int
	}{
		{name: "PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10},
			count: 4},
		{name: "PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true},
			count: 8},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 5, Private: true},
			count: 8},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 2, PageSize: 5, Private: true},
			count: 8},
		{name: "PublicRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15},
			count: 3}, // FIXME: Should return 2 (only directly owned repositories), now includes 1 public repository from owned organization
		{name: "PublicAndPrivateRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true},
			count: 6}, // FIXME: Should return 4 (only directly owned repositories), now includes 2 repositories from owned organization
		{name: "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: true},
			count: 4},
		{name: "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true},
			count: 8},
		{name: "PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17},
			count: 1},
		{name: "PublicAndPrivateRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, Private: true},
			count: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.opts.OwnerID > 0 {
				testCase.opts.Searcher = &User{ID: testCase.opts.OwnerID}
			}
			repos, count, err := SearchRepositoryByName(testCase.opts)

			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)

			var expectedLen int
			if testCase.opts.PageSize*testCase.opts.Page > testCase.count {
				expectedLen = testCase.count % testCase.opts.PageSize
			} else {
				expectedLen = testCase.opts.PageSize
			}
			assert.Len(t, repos, expectedLen)

			for _, repo := range repos {
				assert.NotEmpty(t, repo.Name)

				if len(testCase.opts.Keyword) > 0 {
					assert.Contains(t, repo.Name, testCase.opts.Keyword)
				}

				// FIXME: Can't check, need to fix current behaviour (see previous FIXME comments in test cases)
				/*if testCase.opts.OwnerID > 0 && !testCase.opts.Collaborate {
					assert.Equal(t, testCase.opts.OwnerID, repo.Owner.ID)
				}*/

				if !testCase.opts.Private {
					assert.False(t, repo.IsPrivate)
				}
			}
		})
	}
}
