// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
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

	// Test non existing owner
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{OwnerID: int64(99999)})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	// excepted count by repo type
	type ec map[SearchMode]int

	helperEC := ec{SearchModeAny: 14, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 4, SearchModeCollaborative: 10}
	helperECZero := ec{SearchModeAny: 0, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 0}

	testCases := []struct {
		name  string
		opts  *SearchRepoOptions
		count ec
	}{
		{name: "PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10},
			count: ec{SearchModeAny: 7, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 5}},
		{name: "PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true},
			count: helperEC},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 5, Private: true},
			count: helperEC},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 2, PageSize: 5, Private: true},
			count: helperEC},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitThirdPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true},
			count: helperEC},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFourthPage",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true},
			count: helperEC},
		{name: "PublicRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15},
			count: ec{SearchModeAny: 2, SearchModeSource: 2, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 0}},
		{name: "PublicRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18},
			count: helperECZero},
		{name: "PublicRepositoriesOfUser3",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20},
			count: ec{SearchModeAny: 2, SearchModeSource: 0, SearchModeFork: 1, SearchModeMirror: 1, SearchModeCollaborative: 0}},
		{name: "PublicAndPrivateRepositoriesOfUser",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true},
			count: ec{SearchModeAny: 4, SearchModeSource: 4, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 0}},
		{name: "PublicAndPrivateRepositoriesOfUser2",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true},
			count: helperECZero},
		{name: "PublicAndPrivateRepositoriesOfUser3",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Private: true},
			count: ec{SearchModeAny: 4, SearchModeSource: 0, SearchModeFork: 2, SearchModeMirror: 2, SearchModeCollaborative: 0}},
		{name: "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: true},
			count: ec{SearchModeAny: 4, SearchModeSource: 2, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 2}},
		{name: "PublicRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Collaborate: true},
			count: ec{SearchModeAny: 1, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 1}},
		{name: "PublicRepositoriesOfUser3IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Collaborate: true},
			count: ec{SearchModeAny: 3, SearchModeSource: 0, SearchModeFork: 1, SearchModeMirror: 2, SearchModeCollaborative: 0}},
		{name: "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true},
			count: ec{SearchModeAny: 8, SearchModeSource: 4, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 4}},
		{name: "PublicAndPrivateRepositoriesOfUser2IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 18, Private: true, Collaborate: true},
			count: ec{SearchModeAny: 4, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 4}},
		{name: "PublicAndPrivateRepositoriesOfUser3IncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 20, Private: true, Collaborate: true},
			count: ec{SearchModeAny: 6, SearchModeSource: 0, SearchModeFork: 2, SearchModeMirror: 4, SearchModeCollaborative: 0}},
		{name: "PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17},
			count: ec{SearchModeAny: 1, SearchModeSource: 1, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 0}},
		{name: "PublicAndPrivateRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, Private: true},
			count: ec{SearchModeAny: 2, SearchModeSource: 2, SearchModeFork: 0, SearchModeMirror: 0, SearchModeCollaborative: 0}},
		{name: "AllPublic/PublicRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", PageSize: 10, AllPublic: true},
			count: ec{SearchModeAny: 7, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 5}},
		{name: "AllPublic/PublicAndPrivateRepositoriesByName",
			opts:  &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true, AllPublic: true},
			count: helperEC},
		{name: "AllPublic/PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Collaborate: true, AllPublic: true},
			count: ec{SearchModeAny: 15, SearchModeSource: 2, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 9}},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true, AllPublic: true},
			count: ec{SearchModeAny: 19, SearchModeSource: 4, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 11}},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 15, Private: true, Collaborate: true, AllPublic: true},
			count: ec{SearchModeAny: 13, SearchModeSource: 4, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 7}},
		{name: "AllPublic/PublicAndPrivateRepositoriesOfUser2IncludingCollaborativeByName",
			opts:  &SearchRepoOptions{Keyword: "test", Page: 1, PageSize: 10, OwnerID: 18, Private: true, Collaborate: true, AllPublic: true},
			count: ec{SearchModeAny: 11, SearchModeSource: 0, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 9}},
		{name: "AllPublic/PublicRepositoriesOfOrganization",
			opts:  &SearchRepoOptions{Page: 1, PageSize: 10, OwnerID: 17, AllPublic: true},
			count: ec{SearchModeAny: 15, SearchModeSource: 1, SearchModeFork: 0, SearchModeMirror: 2, SearchModeCollaborative: 10}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for searchMode, expectedCount := range testCase.count {
				testName := "SearchModeAny"
				testCase.opts.SearchMode = searchMode
				if len(searchMode) > 0 {
					testName = fmt.Sprintf("SearchMode%s", searchMode)
				}

				page := testCase.opts.Page
				if page <= 0 {
					page = 1
				}

				t.Run(testName, func(t *testing.T) {
					repos, count, err := SearchRepositoryByName(testCase.opts)

					assert.NoError(t, err)
					assert.Equal(t, int64(expectedCount), count)

					var expectedLen int
					if testCase.opts.PageSize*page > expectedCount+testCase.opts.PageSize {
						expectedLen = 0
					} else if testCase.opts.PageSize*page > expectedCount {
						expectedLen = expectedCount % testCase.opts.PageSize
					} else {
						expectedLen = testCase.opts.PageSize
					}
					if assert.Len(t, repos, expectedLen) {
						var tester func(t *testing.T, ownerID int64, repo *Repository)

						switch searchMode {
						case SearchModeSource:
							tester = SearchModeSourceTester
						case SearchModeFork:
							tester = SearchModeForkTester
						case SearchModeMirror:
							tester = SearchModeMirrorTester
						case SearchModeCollaborative:
							tester = SearchModeCollaborativeTester
						}

						for _, repo := range repos {
							assert.NotEmpty(t, repo.Name)

							if len(testCase.opts.Keyword) > 0 {
								assert.Contains(t, repo.Name, testCase.opts.Keyword)
							}

							if !testCase.opts.Private {
								assert.False(t, repo.IsPrivate)
							}

							if tester != nil {
								tester(t, testCase.opts.OwnerID, repo)
							}
						}
					}
				})
			}
		})
	}
}

func SearchModeSourceTester(t *testing.T, ownerID int64, repo *Repository) {
	assert.False(t, repo.IsFork)
	assert.False(t, repo.IsMirror)
	assert.Equal(t, ownerID, repo.Owner.ID)
}

func SearchModeForkTester(t *testing.T, ownerID int64, repo *Repository) {
	assert.True(t, repo.IsFork)
	assert.False(t, repo.IsMirror)
	assert.Equal(t, ownerID, repo.Owner.ID)
}

func SearchModeMirrorTester(t *testing.T, ownerID int64, repo *Repository) {
	assert.True(t, repo.IsMirror)
}

func SearchModeCollaborativeTester(t *testing.T, ownerID int64, repo *Repository) {
	assert.False(t, repo.IsMirror)
	assert.NotEqual(t, ownerID, repo.Owner.ID)
}
