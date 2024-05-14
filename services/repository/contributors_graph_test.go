// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRepository_ContributorsGraph(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))

	t.Run("non-existent revision", func(t *testing.T) {
		mockCache, err := cache.NewStringCache(setting.Cache{})
		assert.NoError(t, err)
		generateContributorStats(nil, mockCache, "key", repo, "404ref")
		var data map[string]*ContributorData
		_, getErr := mockCache.GetJSON("key", &data)
		assert.NotNil(t, getErr)
		assert.ErrorContains(t, getErr.ToError(), "object does not exist")
	})
	t.Run("generate contributor stats", func(t *testing.T) {
		mockCache, err := cache.NewStringCache(setting.Cache{})
		assert.NoError(t, err)
		generateContributorStats(nil, mockCache, "key", repo, "master")
		var data map[string]*ContributorData
		exist, _ := mockCache.GetJSON("key", &data)
		assert.True(t, exist)
		var keys []string
		for k := range data {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		assert.EqualValues(t, []string{
			"ethantkoenig@gmail.com",
			"jimmy.praet@telenet.be",
			"jon@allspice.io",
			"total", // generated summary
		}, keys)

		assert.EqualValues(t, &ContributorData{
			Name:         "Ethan Koenig",
			AvatarLink:   "https://secure.gravatar.com/avatar/b42fb195faa8c61b8d88abfefe30e9e3?d=identicon",
			TotalCommits: 1,
			Weeks: map[int64]*WeekData{
				1511654400000: {
					Week:      1511654400000, // sunday 2017-11-26
					Additions: 3,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["ethantkoenig@gmail.com"])
		assert.EqualValues(t, &ContributorData{
			Name:         "Total",
			AvatarLink:   "",
			TotalCommits: 3,
			Weeks: map[int64]*WeekData{
				1511654400000: {
					Week:      1511654400000, // sunday 2017-11-26 (2017-11-26 20:31:18 -0800)
					Additions: 3,
					Deletions: 0,
					Commits:   1,
				},
				1607817600000: {
					Week:      1607817600000, // sunday 2020-12-13 (2020-12-15 15:23:11 -0500)
					Additions: 10,
					Deletions: 0,
					Commits:   1,
				},
				1624752000000: {
					Week:      1624752000000, // sunday 2021-06-27 (2021-06-29 21:54:09 +0200)
					Additions: 2,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["total"])
	})
	t.Run("generate contributor stats with co-authored commit", func(t *testing.T) {
		mockCache, err := cache.NewStringCache(setting.Cache{})
		assert.NoError(t, err)
		generateContributorStats(nil, mockCache, "key", repo, "branch-with-co-author")
		var data map[string]*ContributorData
		exist, _ := mockCache.GetJSON("key", &data)
		assert.True(t, exist)
		var keys []string
		for k := range data {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		assert.EqualValues(t, []string{
			"ethantkoenig@gmail.com",
			"fizzbuzz@example.com",
			"foobar@example.com",
			"jimmy.praet@telenet.be",
			"jon@allspice.io",
			"total",
		}, keys)

		// make sure we can see the author of the commit
		assert.EqualValues(t, &ContributorData{
			Name:         "Foo Bar",
			AvatarLink:   "https://secure.gravatar.com/avatar/0d4907cea9d97688aa7a5e722d742f71?d=identicon",
			TotalCommits: 1,
			Weeks: map[int64]*WeekData{
				1714867200000: {
					Week:      1714867200000, // sunday 2024-05-05
					Additions: 1,
					Deletions: 1,
					Commits:   1,
				},
			},
		}, data["foobar@example.com"])

		// make sure that we can also see the co-author
		assert.EqualValues(t, &ContributorData{
			Name:         "Fizz Buzz",
			AvatarLink:   "https://secure.gravatar.com/avatar/474e3516254f43b2337011c4ac4de421?d=identicon",
			TotalCommits: 1,
			Weeks: map[int64]*WeekData{
				1714867200000: {
					Week:      1714867200000, // sunday 2024-05-05
					Additions: 1,
					Deletions: 1,
					Commits:   1,
				},
			},
		}, data["fizzbuzz@example.com"])

		// let's also make sure we don't duplicate the additions/deletions/commits counts in the overall stats that week
		assert.EqualValues(t, &ContributorData{
			Name:         "Total",
			AvatarLink:   "",
			TotalCommits: 4,
			Weeks: map[int64]*WeekData{
				1714867200000: {
					Week:      1714867200000, // sunday 2024-05-05
					Additions: 1,
					Deletions: 1,
					Commits:   1,
				},
				1511654400000: {
					Week:      1511654400000, // sunday 2017-11-26
					Additions: 3,
					Deletions: 0,
					Commits:   1,
				},
				1607817600000: {
					Week:      1607817600000, // sunday 2020-12-13
					Additions: 10,
					Deletions: 0,
					Commits:   1,
				},
				1624752000000: {
					Week:      1624752000000, // sunday 2021-06-27
					Additions: 2,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["total"])
	})
	t.Run("generate contributor stats with commit that has duplicate co-authored lines", func(t *testing.T) {
		mockCache, err := cache.NewStringCache(setting.Cache{})
		assert.NoError(t, err)
		generateContributorStats(nil, mockCache, "key", repo, "branch-with-duplicated-co-author-entries")
		var data map[string]*ContributorData
		exist, _ := mockCache.GetJSON("key", &data)
		assert.True(t, exist)
		var keys []string
		for k := range data {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		assert.EqualValues(t, []string{
			"ethantkoenig@gmail.com",
			"fizzbuzz@example.com",
			"foobar@example.com",
			"jimmy.praet@telenet.be",
			"jon@allspice.io",
			"total",
		}, keys)

		// make sure we can see the author of the commit
		assert.EqualValues(t, &ContributorData{
			Name:         "Foo Bar",
			AvatarLink:   "https://secure.gravatar.com/avatar/0d4907cea9d97688aa7a5e722d742f71?d=identicon",
			TotalCommits: 1,
			Weeks: map[int64]*WeekData{
				1715472000000: {
					Week:      1715472000000, // sunday 2024-05-12
					Additions: 1,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["foobar@example.com"])

		// make sure that we can also see the co-author and that we don't see duplicated additions/deletions/commits
		assert.EqualValues(t, &ContributorData{
			Name:         "Fizz Buzz",
			AvatarLink:   "https://secure.gravatar.com/avatar/474e3516254f43b2337011c4ac4de421?d=identicon",
			TotalCommits: 1,
			Weeks: map[int64]*WeekData{
				1715472000000: {
					Week:      1715472000000, // sunday 2024-05-12
					Additions: 1,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["fizzbuzz@example.com"])

		// let's also make sure we don't duplicate the additions/deletions/commits counts in the overall stats that week
		assert.EqualValues(t, &ContributorData{
			Name:         "Total",
			AvatarLink:   "",
			TotalCommits: 4,
			Weeks: map[int64]*WeekData{
				1715472000000: {
					Week:      1715472000000, // sunday 2024-05-12
					Additions: 1,
					Deletions: 0,
					Commits:   1,
				},
				1511654400000: {
					Week:      1511654400000, // sunday 2017-11-26
					Additions: 3,
					Deletions: 0,
					Commits:   1,
				},
				1607817600000: {
					Week:      1607817600000, // sunday 2020-12-13
					Additions: 10,
					Deletions: 0,
					Commits:   1,
				},
				1624752000000: {
					Week:      1624752000000, // sunday 2021-06-27
					Additions: 2,
					Deletions: 0,
					Commits:   1,
				},
			},
		}, data["total"])
	})
}
