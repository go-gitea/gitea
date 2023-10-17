// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func getTestCases() []struct {
	name  string
	opts  *repo_model.SearchRepoOptions
	count int
} {
	testCases := []struct {
		name  string
		opts  *repo_model.SearchRepoOptions
		count int
	}{
		{
			name:  "PublicRepositoriesByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{PageSize: 10}, Collaborate: util.OptionalBoolFalse},
			count: 7,
		},
		{
			name:  "PublicAndPrivateRepositoriesByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 5}, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 2, PageSize: 5}, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitThirdPage",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 3, PageSize: 5}, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFourthPage",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 3, PageSize: 5}, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "PublicRepositoriesOfUser",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Collaborate: util.OptionalBoolFalse},
			count: 2,
		},
		{
			name:  "PublicRepositoriesOfUser2",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Collaborate: util.OptionalBoolFalse},
			count: 0,
		},
		{
			name:  "PublicRepositoriesOfOrg3",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Collaborate: util.OptionalBoolFalse},
			count: 2,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 4,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser2",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 0,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrg3",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 4,
		},
		{
			name:  "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15},
			count: 5,
		},
		{
			name:  "PublicRepositoriesOfUser2IncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18},
			count: 1,
		},
		{
			name:  "PublicRepositoriesOfOrg3IncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20},
			count: 3,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true},
			count: 9,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser2IncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true},
			count: 4,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrg3IncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Private: true},
			count: 7,
		},
		{
			name:  "PublicRepositoriesOfOrganization",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, Collaborate: util.OptionalBoolFalse},
			count: 1,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrganization",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, Private: true, Collaborate: util.OptionalBoolFalse},
			count: 2,
		},
		{
			name:  "AllPublic/PublicRepositoriesByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{PageSize: 10}, AllPublic: true, Collaborate: util.OptionalBoolFalse},
			count: 7,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, AllPublic: true, Collaborate: util.OptionalBoolFalse},
			count: 14,
		},
		{
			name:  "AllPublic/PublicRepositoriesOfUserIncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, AllPublic: true, Template: util.OptionalBoolFalse},
			count: 31,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, AllPublic: true, AllLimited: true, Template: util.OptionalBoolFalse},
			count: 36,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborativeByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "test", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, AllPublic: true},
			count: 15,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUser2IncludingCollaborativeByName",
			opts:  &repo_model.SearchRepoOptions{Keyword: "test", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true, AllPublic: true},
			count: 13,
		},
		{
			name:  "AllPublic/PublicRepositoriesOfOrganization",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, AllPublic: true, Collaborate: util.OptionalBoolFalse, Template: util.OptionalBoolFalse},
			count: 31,
		},
		{
			name:  "AllTemplates",
			opts:  &repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Template: util.OptionalBoolTrue},
			count: 2,
		},
		{
			name:  "OwnerSlashRepoSearch",
			opts:  &repo_model.SearchRepoOptions{Keyword: "user/repo2", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, OwnerID: 0},
			count: 2,
		},
		{
			name:  "OwnerSlashSearch",
			opts:  &repo_model.SearchRepoOptions{Keyword: "user20/", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, OwnerID: 0},
			count: 4,
		},
	}

	return testCases
}

func TestSearchRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// test search public repository on explore page
	repos, count, err := repo_model.SearchRepositoryByName(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "repo_12",
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_12", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = repo_model.SearchRepositoryByName(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "test_repo",
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)

	// test search private repository on explore page
	repos, count, err = repo_model.SearchRepositoryByName(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "repo_13",
		Private:     true,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_13", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = repo_model.SearchRepositoryByName(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "test_repo",
		Private:     true,
		Collaborate: util.OptionalBoolFalse,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, repos, 3)

	// Test non existing owner
	repos, count, err = repo_model.SearchRepositoryByName(db.DefaultContext, &repo_model.SearchRepoOptions{OwnerID: unittest.NonexistentID})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	// Test search within description
	repos, count, err = repo_model.SearchRepository(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:            "description_14",
		Collaborate:        util.OptionalBoolFalse,
		IncludeDescription: true,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_14", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	// Test NOT search within description
	repos, count, err = repo_model.SearchRepository(db.DefaultContext, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:            "description_14",
		Collaborate:        util.OptionalBoolFalse,
		IncludeDescription: false,
	})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)

	testCases := getTestCases()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repos, count, err := repo_model.SearchRepositoryByName(db.DefaultContext, testCase.opts)

			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)

			page := testCase.opts.Page
			if page <= 0 {
				page = 1
			}
			expectedLen := testCase.opts.PageSize
			if testCase.opts.PageSize*page > testCase.count+testCase.opts.PageSize {
				expectedLen = 0
			} else if testCase.opts.PageSize*page > testCase.count {
				expectedLen = testCase.count % testCase.opts.PageSize
			}
			if assert.Len(t, repos, expectedLen) {
				for _, repo := range repos {
					assert.NotEmpty(t, repo.Name)

					if len(testCase.opts.Keyword) > 0 {
						// Keyword match condition is different for search terms of form "owner/repo"
						if strings.Count(testCase.opts.Keyword, "/") == 1 {
							// May still match as a whole...
							wholeMatch := strings.Contains(repo.Name, testCase.opts.Keyword)

							pieces := strings.Split(testCase.opts.Keyword, "/")
							ownerName := pieces[0]
							repoName := pieces[1]
							// ... or match in parts
							splitMatch := strings.Contains(repo.OwnerName, ownerName) && strings.Contains(repo.Name, repoName)

							assert.True(t, wholeMatch || splitMatch, "Keyword '%s' does not match repo '%s/%s'", testCase.opts.Keyword, repo.Owner.Name, repo.Name)
						} else {
							assert.Contains(t, repo.Name, testCase.opts.Keyword)
						}
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

func TestCountRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testCases := getTestCases()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			count, err := repo_model.CountRepository(db.DefaultContext, testCase.opts)

			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)
		})
	}
}

func TestSearchRepositoryByTopicName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testCases := []struct {
		name  string
		opts  *repo_model.SearchRepoOptions
		count int
	}{
		{
			name:  "AllPublic/SearchPublicRepositoriesFromTopicAndName",
			opts:  &repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql"},
			count: 2,
		},
		{
			name:  "AllPublic/OnlySearchPublicRepositoriesFromTopic",
			opts:  &repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql", TopicOnly: true},
			count: 1,
		},
		{
			name:  "AllPublic/OnlySearchMultipleKeywordPublicRepositoriesFromTopic",
			opts:  &repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql,golang", TopicOnly: true},
			count: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, count, err := repo_model.SearchRepositoryByName(db.DefaultContext, testCase.opts)
			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)
		})
	}
}
