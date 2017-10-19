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
			count: 2},
		{name: "PublicRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18},
			count: 0},
		{name: "PublicAndPrivateRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true},
			count: 4},
		{name: "PublicAndPrivateRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true},
			count: 0},
		{name: "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: true},
			count: 4},
		{name: "PublicRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Collaborate: true},
			count: 1},
		{name: "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true},
			count: 8},
		{name: "PublicAndPrivateRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true, Collaborate: true},
			count: 4},
		{name: "PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17},
			count: 1},
		{name: "PublicAndPrivateRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, Private: true},
			count: 2},
		{name: "AllPublic/PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10, AllPublic: true},
			count: 4},
		{name: "AllPublic/PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true, AllPublic: true},
			count: 8},
		{name: "AllPublic/PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: true, AllPublic: true},
			count: 12},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true, AllPublic: true},
			count: 16},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true, AllPublic: true},
			count: 10},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUser2IncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 18, Private: true, Collaborate: true, AllPublic: true},
			count: 8},
		{name: "AllPublic/PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, AllPublic: true},
			count: 12},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
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

				if testCase.opts.OwnerID > 0 && !testCase.opts.Collaborate && !testCase.opts.AllPublic {
					assert.Equal(t, testCase.opts.OwnerID, repo.Owner.ID)
				}

				if !testCase.opts.Private {
					assert.False(t, repo.IsPrivate)
				}
			}
		})
	}
}
