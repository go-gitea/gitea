// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestSearchRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// test search public repository on explore page
	repos, count, err := SearchRepositoryByName(&SearchRepoOptions{
		Keyword:     "repo_12",
		Page:        1,
		PageSize:    10,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_12", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:     "test_repo",
		Page:        1,
		PageSize:    10,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)

	// test search private repository on explore page
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:     "repo_13",
		Page:        1,
		PageSize:    10,
		Private:     true,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_13", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		Keyword:     "test_repo",
		Page:        1,
		PageSize:    10,
		Private:     true,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, repos, 3)

	// Test non existing owner
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{OwnerID: NonexistentID})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	// Test search within description
	repos, count, err = SearchRepository(&SearchRepoOptions{
		Keyword:            "description_14",
		Page:               1,
		PageSize:           10,
		Collaborate:        util.OptionalBoolFalse,
		IncludeDescription: true,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_14", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	// Test NOT search within description
	repos, count, err = SearchRepository(&SearchRepoOptions{
		Keyword:            "description_14",
		Page:               1,
		PageSize:           10,
		Collaborate:        util.OptionalBoolFalse,
		IncludeDescription: false,
	})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	testCases := []struct {
		name  string
		opts  *SearchRepoOptions
		count int
	}{
		{name: "PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10, Collaborate: util.OptionalBoolFalse},
			count: 7},
		{name: "PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 5, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 2, PageSize: 5, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitThirdPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFourthPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "PublicRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: util.OptionalBoolFalse},
			count: 2},
		{name: "PublicRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Collaborate: util.OptionalBoolFalse},
			count: 0},
		{name: "PublicRepositoriesOfUser3",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Collaborate: util.OptionalBoolFalse},
			count: 2},
		{name: "PublicAndPrivateRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 4},
		{name: "PublicAndPrivateRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 0},
		{name: "PublicAndPrivateRepositoriesOfUser3",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 4},
		{name: "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15},
			count: 5},
		{name: "PublicRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18},
			count: 1},
		{name: "PublicRepositoriesOfUser3IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20},
			count: 3},
		{name: "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true},
			count: 9},
		{name: "PublicAndPrivateRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true},
			count: 4},
		{name: "PublicAndPrivateRepositoriesOfUser3IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Private: true},
			count: 7},
		{name: "PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, Collaborate: util.OptionalBoolFalse},
			count: 1},
		{name: "PublicAndPrivateRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 2},
		{name: "AllPublic/PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10, AllPublic: true, Collaborate: util.OptionalBoolFalse},
			count: 7},
		{name: "AllPublic/PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true, AllPublic: true, Collaborate: util.OptionalBoolFalse},
			count: 14},
		{name: "AllPublic/PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, AllPublic: true, Template: util.OptionalBoolFalse},
			count: 25},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, AllPublic: true, AllLimited: true, Template: util.OptionalBoolFalse},
			count: 30},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 15, Private: true, AllPublic: true},
			count: 15},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUser2IncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 18, Private: true, AllPublic: true},
			count: 13},
		{name: "AllPublic/PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, AllPublic: true, Collaborate: util.OptionalBoolFalse, Template: util.OptionalBoolFalse},
			count: 25},
		{name: "AllTemplates",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, Template: util.OptionalBoolTrue},
			count: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repos, count, err := SearchRepositoryByName(testCase.opts)

			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)

			page := testCase.opts.Page
			if page <= 0 {
				page = 1
			}
			var expectedLen = testCase.opts.PageSize
			if testCase.opts.PageSize*page > testCase.count+testCase.opts.PageSize {
				expectedLen = 0
			} else if testCase.opts.PageSize*page > testCase.count {
				expectedLen = testCase.count % testCase.opts.PageSize
			}
			if assert.Len(t, repos, expectedLen) {
				for _, repo := range repos {
					assert.NotEmpty(t, repo.Name)

					if len(testCase.opts.Keyword) > 0 {
						assert.Contains(t, repo.Name, testCase.opts.Keyword)
					}

					if !testCase.opts.Private {
						assert.False(t, repo.IsPrivate)
					}

					if testCase.opts.Fork == util.OptionalBoolTrue && testCase.opts.Mirror == util.OptionalBoolTrue {
						assert.True(t, repo.IsFork || repo.IsMirror)
					} else {
						switch testCase.opts.Fork {
						case util.OptionalBoolFalse:
							assert.False(t, repo.IsFork)
						case util.OptionalBoolTrue:
							assert.True(t, repo.IsFork)
						}

						switch testCase.opts.Mirror {
						case util.OptionalBoolFalse:
							assert.False(t, repo.IsMirror)
						case util.OptionalBoolTrue:
							assert.True(t, repo.IsMirror)
						}
					}

					if testCase.opts.OwnerID > 0 && !testCase.opts.AllPublic {
						switch testCase.opts.Collaborate {
						case util.OptionalBoolFalse:
							assert.Equal(t, testCase.opts.OwnerID, repo.Owner.ID)
						case util.OptionalBoolTrue:
							assert.NotEqual(t, testCase.opts.OwnerID, repo.Owner.ID)
						}
					}
				}
			}
		})
	}
}

func TestSearchRepositoryByTopicName(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testCases := []struct {
		name  string
		opts  *SearchRepoOptions
		count int
	}{
		{name: "AllPublic/SearchPublicRepositoriesFromTopicAndName",
			opts:  &SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql"},
			count: 2},
		{name: "AllPublic/OnlySearchPublicRepositoriesFromTopic",
			opts:  &SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql", TopicOnly: true},
			count: 1},
		{name: "AllPublic/OnlySearchMultipleKeywordPublicRepositoriesFromTopic",
			opts:  &SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql,golang", TopicOnly: true},
			count: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, count, err := SearchRepositoryByName(testCase.opts)
			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)
		})
	}
}
