// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"slices"
	"strings"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	contribution_model "gitea.dev/models/repo/contribution"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepository_ContributorsGraph(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(t.Context()))
	assert.NoError(t, contribution_model.DeleteRepoContributorDailyStats(t.Context(), repo.ID))

	var data map[string]*ContributorData
	_, err := GetContributorStats(t.Context(), repo, 0, nil, nil)
	assert.ErrorIs(t, err, ErrAwaitGeneration)

	assert.NoError(t, processContributorStatsRebuild(t.Context(), &ContributorStatsRebuildOptions{RepoID: repo.ID}))

	data, err = GetContributorStats(t.Context(), repo, 0, nil, nil)
	assert.NoError(t, err)
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
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
		Weeks: map[int64]*contribution_model.WeekData{
			1511654400000: {
				Week:         1511654400000, // sunday 2017-11-26
				Additions:    3,
				Deletions:    0,
				Commits:      1,
				ChangedFiles: 1,
			},
		},
	}, data["ethantkoenig@gmail.com"])
	assert.Equal(t, &ContributorData{
		Name:         "Total",
		AvatarLink:   "",
		TotalCommits: 3,
		Weeks: map[int64]*contribution_model.WeekData{
			1511654400000: {
				Week:         1511654400000, // sunday 2017-11-26 (2017-11-26 20:31:18 -0800)
				Additions:    3,
				Deletions:    0,
				Commits:      1,
				ChangedFiles: 1,
			},
			1607817600000: {
				Week:         1607817600000, // sunday 2020-12-13 (2020-12-15 15:23:11 -0500)
				Additions:    10,
				Deletions:    0,
				Commits:      1,
				ChangedFiles: 1,
			},
			1624752000000: {
				Week:         1624752000000, // sunday 2021-06-27 (2021-06-29 21:54:09 +0200)
				Additions:    2,
				Deletions:    0,
				Commits:      1,
				ChangedFiles: 1,
			},
		},
	}, data["total"])
}

func TestScanOneStat(t *testing.T) {
	input := strings.Join([]string{
		"---",
		"abc123",
		"Author Name",
		"author@example.com",
		"2026-04-16T04:07:57+08:00",
		"",
		"  9902 files changed, 2034198 insertions(+), 298800 deletions(-)",
		"",
		"---",
		"def456",
		"Second Author",
		"second@example.com",
		"2026-04-17T05:07:57+08:00",
		"",
		"  5 files changed, 1 insertions(+), 1 deletions(-)",
		"",
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))

	commitID, authorName, email, date, additions, deletions, changedFiles, err := scanOneStat(scanner)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", commitID)
	assert.Equal(t, "Author Name", authorName)
	assert.Equal(t, "author@example.com", email)
	if assert.NotNil(t, date) {
		expectedDate, err := time.Parse(time.RFC3339, "2026-04-16T04:07:57+08:00")
		assert.NoError(t, err)
		assert.True(t, date.Equal(expectedDate))
	}
	assert.Equal(t, int64(2034198), additions)
	assert.Equal(t, int64(298800), deletions)
	assert.Equal(t, int64(9902), changedFiles)

	commitID, authorName, email, date, additions, deletions, changedFiles, err = scanOneStat(scanner)
	assert.NoError(t, err)
	assert.Equal(t, "def456", commitID)
	assert.Equal(t, "Second Author", authorName)
	assert.Equal(t, "second@example.com", email)
	if assert.NotNil(t, date) {
		expectedDate, err := time.Parse(time.RFC3339, "2026-04-17T05:07:57+08:00")
		assert.NoError(t, err)
		assert.True(t, date.Equal(expectedDate))
	}
	assert.Equal(t, int64(1), additions)
	assert.Equal(t, int64(1), deletions)
	assert.Equal(t, int64(5), changedFiles)

	_, _, _, _, _, _, _, err = scanOneStat(scanner)
	assert.ErrorIs(t, err, errEndOfGitLogOutput)
}

func TestGetExtendedCommitStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	stats, err := getExtendedCommitStats(t.Context(), repo, repo.DefaultBranch)
	assert.NoError(t, err)
	if assert.NotEmpty(t, stats) {
		foundChangedFiles := false
		for _, stat := range stats {
			if stat == nil || stat.Stats == nil || stat.Author == nil {
				continue
			}
			if stat.Stats.ChangedFiles > 0 {
				foundChangedFiles = true
				break
			}
		}
		assert.True(t, foundChangedFiles)
	}
}
