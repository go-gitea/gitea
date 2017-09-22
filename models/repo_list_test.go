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
	nonExistingUserID := int64(99999)
	repos, count, err = SearchRepositoryByName(&SearchRepoOptions{
		OwnerID: nonExistingUserID,
	})

	if assert.Error(t, err) {
		assert.Equal(t, ErrUserNotExist{UID: nonExistingUserID}, err)
	}
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	type expectedCounts map[RepoType]int

	helperExpectedCounts := expectedCounts{RepoTypeAny: 14, RepoTypeSource: 8, RepoTypeFork: 2, RepoTypeMirror: 4, RepoTypeCollaborative: 0}

	testCases := []struct {
		name string
		opts *SearchRepoOptions
		expectedCounts
	}{
		{name: "PublicRepositoriesByName",
			opts:           &SearchRepoOptions{Keyword: "big_test_", PageSize: 10},
			expectedCounts: expectedCounts{RepoTypeAny: 7, RepoTypeSource: 4, RepoTypeFork: 1, RepoTypeMirror: 2, RepoTypeCollaborative: 0},
		},
		{name: "PublicAndPrivateRepositoriesByName",
			opts:           &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 10, Private: true},
			expectedCounts: helperExpectedCounts,
		},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:           &SearchRepoOptions{Keyword: "big_test_", Page: 1, PageSize: 5, Private: true},
			expectedCounts: helperExpectedCounts,
		},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:           &SearchRepoOptions{Keyword: "big_test_", Page: 2, PageSize: 5, Private: true},
			expectedCounts: helperExpectedCounts,
		},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitThirdPage",
			opts:           &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true},
			expectedCounts: helperExpectedCounts,
		},
		{name: "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFourthPage",
			opts:           &SearchRepoOptions{Keyword: "big_test_", Page: 3, PageSize: 5, Private: true},
			expectedCounts: helperExpectedCounts,
		},
		{name: "PublicRepositoriesOfUser",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 15},
			expectedCounts: expectedCounts{RepoTypeAny: 4, RepoTypeSource: 2, RepoTypeFork: 1, RepoTypeMirror: 1, RepoTypeCollaborative: 0},
		},
		{name: "PublicAndPrivateRepositoriesOfUser",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 15, Private: true},
			expectedCounts: expectedCounts{RepoTypeAny: 8, RepoTypeSource: 4, RepoTypeFork: 2, RepoTypeMirror: 2, RepoTypeCollaborative: 0},
		},
		{name: "PublicRepositoriesOfUserIncludingCollaborative",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 15, Collaborate: true},
			expectedCounts: expectedCounts{RepoTypeAny: 7, RepoTypeSource: 2, RepoTypeFork: 1, RepoTypeMirror: 2, RepoTypeCollaborative: 2},
		},
		{name: "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 15, Private: true, Collaborate: true},
			expectedCounts: expectedCounts{RepoTypeAny: 14, RepoTypeSource: 4, RepoTypeFork: 2, RepoTypeMirror: 4, RepoTypeCollaborative: 4},
		},
		{name: "PublicRepositoriesOfOrganization",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 17},
			expectedCounts: expectedCounts{RepoTypeAny: 2, RepoTypeSource: 1, RepoTypeFork: 0, RepoTypeMirror: 1, RepoTypeCollaborative: 0},
		},
		{name: "PublicAndPrivateRepositoriesOfOrganization",
			opts:           &SearchRepoOptions{Page: 1, PageSize: 50, OwnerID: 17, Private: true},
			expectedCounts: expectedCounts{RepoTypeAny: 4, RepoTypeSource: 2, RepoTypeFork: 0, RepoTypeMirror: 2, RepoTypeCollaborative: 0},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for repoType, expectedCount := range testCase.expectedCounts {
				testName := "RepoTypeANY"
				testCase.opts.RepoType = repoType
				if len(repoType) > 0 {
					testName = fmt.Sprintf("RepoType%s", repoType)
				}

				t.Run(testName, func(t *testing.T) {
					repos, count, err := SearchRepositoryByName(testCase.opts)

					assert.NoError(t, err)
					assert.Equal(t, int64(expectedCount), count)

					var expectedLen int
					if testCase.opts.PageSize*testCase.opts.Page > expectedCount+testCase.opts.PageSize {
						expectedLen = 0
					} else if testCase.opts.PageSize*testCase.opts.Page > expectedCount {
						expectedLen = expectedCount % testCase.opts.PageSize
					} else {
						expectedLen = testCase.opts.PageSize
					}
					if assert.Len(t, repos, expectedLen) {
						var tester func(t *testing.T, ownerID int64, repo *Repository)

						switch repoType {
						case RepoTypeSource:
							tester = repoTypeSourceTester
						case RepoTypeFork:
							tester = repoTypeForkTester
						case RepoTypeMirror:
							tester = repoTypeMirrorTester
						case RepoTypeCollaborative:
							tester = repoTypeCollaborativeTester
						case RepoTypeAny:
							tester = repoTypeDefaultTester
						}

						for _, repo := range repos {
							tester(t, testCase.opts.OwnerID, repo)
						}
					}
				})
			}
		})
	}
}

func repoTypeSourceTester(t *testing.T, ownerID int64, repo *Repository) {
	repoTypeDefaultTester(t, ownerID, repo)
	assert.False(t, repo.IsFork)
	assert.False(t, repo.IsMirror)
	if ownerID > 0 {
		assert.Equal(t, ownerID, repo.OwnerID)
	}
}

func repoTypeForkTester(t *testing.T, ownerID int64, repo *Repository) {
	repoTypeDefaultTester(t, ownerID, repo)
	assert.True(t, repo.IsFork)
	assert.False(t, repo.IsMirror)
	if ownerID > 0 {
		assert.Equal(t, ownerID, repo.OwnerID)
	}
}

func repoTypeMirrorTester(t *testing.T, ownerID int64, repo *Repository) {
	repoTypeDefaultTester(t, ownerID, repo)
	assert.True(t, repo.IsMirror)
}

func repoTypeCollaborativeTester(t *testing.T, ownerID int64, repo *Repository) {
	repoTypeDefaultTester(t, ownerID, repo)
	assert.False(t, repo.IsMirror)
	if ownerID > 0 {
		assert.NotEqual(t, ownerID, repo.OwnerID)
	}
}

func repoTypeDefaultTester(t *testing.T, ownerID int64, repo *Repository) {
	assert.NotEmpty(t, repo.Name)
}
