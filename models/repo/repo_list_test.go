// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestCases() []struct {
	name  string
	opts  repo_model.SearchRepoOptions
	count int
} {
	testCases := []struct {
		name  string
		opts  repo_model.SearchRepoOptions
		count int
	}{
		{
			name:  "PublicRepositoriesByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{PageSize: 10}, Collaborate: optional.Some(false)},
			count: 7,
		},
		{
			name:  "PublicAndPrivateRepositoriesByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFirstPage",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 5}, Private: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitSecondPage",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 2, PageSize: 5}, Private: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitThirdPage",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 3, PageSize: 5}, Private: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "PublicAndPrivateRepositoriesByNameWithPagesizeLimitFourthPage",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 3, PageSize: 5}, Private: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "PublicRepositoriesOfUser",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Collaborate: optional.Some(false)},
			count: 2,
		},
		{
			name:  "PublicRepositoriesOfUser2",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Collaborate: optional.Some(false)},
			count: 0,
		},
		{
			name:  "PublicRepositoriesOfOrg3",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Collaborate: optional.Some(false)},
			count: 2,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, Collaborate: optional.Some(false)},
			count: 4,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser2",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true, Collaborate: optional.Some(false)},
			count: 0,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrg3",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Private: true, Collaborate: optional.Some(false)},
			count: 4,
		},
		{
			name:  "PublicRepositoriesOfUserIncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15},
			count: 5,
		},
		{
			name:  "PublicRepositoriesOfUser2IncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18},
			count: 1,
		},
		{
			name:  "PublicRepositoriesOfOrg3IncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20},
			count: 3,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true},
			count: 9,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfUser2IncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true},
			count: 4,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrg3IncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 20, Private: true},
			count: 7,
		},
		{
			name:  "PublicRepositoriesOfOrganization",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, Collaborate: optional.Some(false)},
			count: 1,
		},
		{
			name:  "PublicAndPrivateRepositoriesOfOrganization",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, Private: true, Collaborate: optional.Some(false)},
			count: 2,
		},
		{
			name:  "AllPublic/PublicRepositoriesByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{PageSize: 10}, AllPublic: true, Collaborate: optional.Some(false)},
			count: 7,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "big_test_", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, AllPublic: true, Collaborate: optional.Some(false)},
			count: 14,
		},
		{
			name:  "AllPublic/PublicRepositoriesOfUserIncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, AllPublic: true, Template: optional.Some(false)},
			count: 34,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborative",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, AllPublic: true, AllLimited: true, Template: optional.Some(false)},
			count: 39,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUserIncludingCollaborativeByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "test", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 15, Private: true, AllPublic: true},
			count: 15,
		},
		{
			name:  "AllPublic/PublicAndPrivateRepositoriesOfUser2IncludingCollaborativeByName",
			opts:  repo_model.SearchRepoOptions{Keyword: "test", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 18, Private: true, AllPublic: true},
			count: 13,
		},
		{
			name:  "AllPublic/PublicRepositoriesOfOrganization",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, OwnerID: 17, AllPublic: true, Collaborate: optional.Some(false), Template: optional.Some(false)},
			count: 34,
		},
		{
			name:  "AllTemplates",
			opts:  repo_model.SearchRepoOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Template: optional.Some(true)},
			count: 2,
		},
		{
			name:  "OwnerSlashRepoSearch",
			opts:  repo_model.SearchRepoOptions{Keyword: "user/repo2", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, OwnerID: 0},
			count: 2,
		},
		{
			name:  "OwnerSlashSearch",
			opts:  repo_model.SearchRepoOptions{Keyword: "user20/", ListOptions: db.ListOptions{Page: 1, PageSize: 10}, Private: true, OwnerID: 0},
			count: 4,
		},
	}

	return testCases
}

func TestSearchRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("SearchRepositoryPublic", testSearchRepositoryPublic)
	t.Run("SearchRepositoryPublicRestricted", testSearchRepositoryRestricted)
	t.Run("SearchRepositoryPrivate", testSearchRepositoryPrivate)
	t.Run("SearchRepositoryNonExistingOwner", testSearchRepositoryNonExistingOwner)
	t.Run("SearchRepositoryWithInDescription", testSearchRepositoryWithInDescription)
	t.Run("SearchRepositoryNotInDescription", testSearchRepositoryNotInDescription)
	t.Run("SearchRepositoryCases", testSearchRepositoryCases)
}

func testSearchRepositoryPublic(t *testing.T) {
	// test search public repository on explore page
	repos, count, err := repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "repo_12",
		Collaborate: optional.Some(false),
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_12", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "test_repo",
		Collaborate: optional.Some(false),
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, repos, 2)
}

func testSearchRepositoryRestricted(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	restrictedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29, IsRestricted: true})

	performSearch := func(t *testing.T, user *user_model.User) (publicRepoIDs []int64, totalCount int) {
		repos, count, err := repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{Page: 1, PageSize: 10000},
			Actor:       user,
		})
		require.Nil(t, err)
		totalCount = int(count)
		assert.Len(t, repos, totalCount)
		for _, repo := range repos {
			require.NoError(t, repo.LoadOwner(t.Context()))
			if repo.Owner.Visibility == structs.VisibleTypePublic && !repo.IsPrivate {
				publicRepoIDs = append(publicRepoIDs, repo.ID)
			}
		}
		return publicRepoIDs, totalCount
	}

	normalUserPublicRepoIDs, totalCount := performSearch(t, user2)
	assert.Positive(t, totalCount)
	assert.Greater(t, len(normalUserPublicRepoIDs), 1) // quite a lot

	restrictedUserPublicRepoIDs, totalCount := performSearch(t, restrictedUser)
	assert.Equal(t, 1, totalCount) // restricted user can see only their own repo
	assert.Equal(t, []int64{4}, restrictedUserPublicRepoIDs)
}

func testSearchRepositoryPrivate(t *testing.T) {
	// test search private repository on explore page
	repos, count, err := repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "repo_13",
		Private:     true,
		Collaborate: optional.Some(false),
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_13", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)

	repos, count, err = repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:     "test_repo",
		Private:     true,
		Collaborate: optional.Some(false),
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, repos, 3)
}

func testSearchRepositoryNonExistingOwner(t *testing.T) {
	repos, count, err := repo_model.SearchRepositoryByName(t.Context(), repo_model.SearchRepoOptions{OwnerID: unittest.NonexistentID})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)
}

func testSearchRepositoryWithInDescription(t *testing.T) {
	repos, count, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:            "description_14",
		Collaborate:        optional.Some(false),
		IncludeDescription: true,
	})

	assert.NoError(t, err)
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "test_repo_14", repos[0].Name)
	}
	assert.Equal(t, int64(1), count)
}

func testSearchRepositoryNotInDescription(t *testing.T) {
	repos, count, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Keyword:            "description_14",
		Collaborate:        optional.Some(false),
		IncludeDescription: false,
	})

	assert.NoError(t, err)
	assert.Empty(t, repos)
	assert.Equal(t, int64(0), count)
}

func testSearchRepositoryCases(t *testing.T) {
	testCases := getTestCases()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repos, count, err := repo_model.SearchRepositoryByName(t.Context(), testCase.opts)

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

					if testCase.opts.Fork.Value() && testCase.opts.Mirror.Value() {
						assert.True(t, repo.IsFork && repo.IsMirror)
					} else {
						if testCase.opts.Fork.Has() {
							assert.Equal(t, testCase.opts.Fork.Value(), repo.IsFork)
						}

						if testCase.opts.Mirror.Has() {
							assert.Equal(t, testCase.opts.Mirror.Value(), repo.IsMirror)
						}
					}

					if testCase.opts.OwnerID > 0 && !testCase.opts.AllPublic {
						if testCase.opts.Collaborate.Has() {
							if testCase.opts.Collaborate.Value() {
								assert.NotEqual(t, testCase.opts.OwnerID, repo.Owner.ID)
							} else {
								assert.Equal(t, testCase.opts.OwnerID, repo.Owner.ID)
							}
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
			count, err := repo_model.CountRepository(t.Context(), testCase.opts)

			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)
		})
	}
}

func TestSearchRepositoryByTopicName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testCases := []struct {
		name  string
		opts  repo_model.SearchRepoOptions
		count int
	}{
		{
			name:  "AllPublic/SearchPublicRepositoriesFromTopicAndName",
			opts:  repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql"},
			count: 2,
		},
		{
			name:  "AllPublic/OnlySearchPublicRepositoriesFromTopic",
			opts:  repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql", TopicOnly: true},
			count: 1,
		},
		{
			name:  "AllPublic/OnlySearchMultipleKeywordPublicRepositoriesFromTopic",
			opts:  repo_model.SearchRepoOptions{OwnerID: 21, AllPublic: true, Keyword: "graphql,golang", TopicOnly: true},
			count: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, count, err := repo_model.SearchRepositoryByName(t.Context(), testCase.opts)
			assert.NoError(t, err)
			assert.Equal(t, int64(testCase.count), count)
		})
	}
}
