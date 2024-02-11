// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"

	"gitea.com/go-chi/cache"
	"github.com/stretchr/testify/assert"
)

func TestRepository_ContributorsGraph(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))
	mockCache, err := cache.NewCacher(cache.Options{
		Adapter:  "memory",
		Interval: 24 * 60,
	})
	assert.NoError(t, err)

	generateContributorStats(nil, mockCache, "key", repo, "404ref")
	err, isErr := mockCache.Get("key").(error)
	assert.True(t, isErr)
	assert.ErrorAs(t, err, &git.ErrNotExist{})

	generateContributorStats(nil, mockCache, "key2", repo, "master")
	data, isData := mockCache.Get("key2").(map[string]*ContributorData)
	assert.True(t, isData)
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
		Weeks: []*WeekData{{
			Week:      1511654400, // sunday 2017-11-26
			Additions: 3,
			Deletions: 0,
			Commits:   1,
		}},
	}, data["ethantkoenig@gmail.com"])
	assert.EqualValues(t, &ContributorData{
		Name:         "total",
		TotalCommits: 3,
		Weeks: []*WeekData{
			{
				Week:      1511654400, // sunday 2017-11-26 (2017-11-26 20:31:18 -0800)
				Additions: 3,
				Deletions: 0,
				Commits:   1,
			},
			{
				Week:      1607817600, // sunday 2020-12-13 (2020-12-15 15:23:11 -0500)
				Additions: 10,
				Deletions: 0,
				Commits:   1,
			},
			{
				Week:      1624752000, // sunday 2021-06-27 (2021-06-29 21:54:09 +0200)
				Additions: 2,
				Deletions: 0,
				Commits:   1,
			},
		},
	}, data["total"])
}
