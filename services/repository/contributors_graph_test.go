// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"maps"
	"slices"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_ContributorsGraph(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.NoError(t, UpdateRepoContributors(t.Context(), repo.ID))

	data, err := GetContributorStats(t.Context(), repo)
	require.NoError(t, err)
	keys := slices.Sorted(maps.Keys(data))
	assert.Equal(t, []string{
		"ethantkoenig@gmail.com",
		"jimmy.praet@telenet.be",
		"jon@allspice.io",
		"total", // generated summary
	}, keys)

	assert.Equal(t, &ContributorData{
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
	assert.Equal(t, &ContributorData{
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
