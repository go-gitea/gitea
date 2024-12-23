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
	mockCache, err := cache.NewStringCache(setting.Cache{})
	assert.NoError(t, err)

	generateContributorStats(nil, mockCache, "key", repo, "404ref")
	var data map[string]*ContributorData
	_, getErr := mockCache.GetJSON("key", &data)
	assert.NotNil(t, getErr)
	assert.ErrorContains(t, getErr.ToError(), "object does not exist")

	generateContributorStats(nil, mockCache, "key2", repo, "master")
	exist, _ := mockCache.GetJSON("key2", &data)
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
		AvatarLink:   "/assets/img/avatar_default.png",
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
}
