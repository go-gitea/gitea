// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRepoContributors(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 16
	const headID = "69554a64c1e6030f051e5c3f94bfbd773cd6a324"
	const parentID = "27566bd5738fc8b4e3fef3c5e72cce608537bd95"
	const week = 1501977600000 // sunday 2017-08-06
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	countedContributors := []*repo_model.RepoContributor{
		{RepoID: repoID, UserID: 2, Commits: 2, Additions: 2, Deletions: 1},
		{RepoID: repoID, UserID: 21, Commits: 1, Additions: 1, Deletions: 1},
	}
	countedWeeks := []*repo_model.RepoContributorWeek{
		{RepoID: repoID, UserID: 2, Week: week, Commits: 2, Additions: 2, Deletions: 1},
		{RepoID: repoID, UserID: 21, Week: week, Commits: 1, Additions: 1, Deletions: 1},
	}
	// not in the git history, so it only survives updates that add to the existing stats
	marker := &repo_model.RepoContributor{RepoID: repoID, Email: "marker@example.com", Name: "Marker", Commits: 9}

	assertStats := func(t *testing.T, contributors []*repo_model.RepoContributor) {
		status, err := repo_model.GetIndexerStatus(t.Context(), repo, repo_model.RepoIndexerTypeContributors)
		require.NoError(t, err)
		assert.Equal(t, headID, status.CommitSha)

		var actualContributors []*repo_model.RepoContributor
		require.NoError(t, db.GetEngine(t.Context()).Where("repo_id = ?", repoID).Asc("user_id", "email").Find(&actualContributors))
		for _, c := range actualContributors {
			c.ID = 0
		}
		assert.Equal(t, contributors, actualContributors)

		var actualWeeks []*repo_model.RepoContributorWeek
		require.NoError(t, db.GetEngine(t.Context()).Where("repo_id = ?", repoID).Asc("user_id", "email").Find(&actualWeeks))
		for _, w := range actualWeeks {
			w.ID = 0
		}
		assert.Equal(t, countedWeeks, actualWeeks)
	}

	setStats := func(t *testing.T, commitID string, contributors []*repo_model.RepoContributor, weeks []*repo_model.RepoContributorWeek) {
		require.NoError(t, repo_model.ReplaceRepoContributors(t.Context(), repoID, contributors, weeks))
		require.NoError(t, repo_model.UpdateIndexerStatus(t.Context(), repo, repo_model.RepoIndexerTypeContributors, commitID))
	}

	t.Run("FirstUpdate", func(t *testing.T) {
		require.NoError(t, UpdateRepoContributors(t.Context(), repoID))
		assertStats(t, countedContributors)
	})

	t.Run("FastForward", func(t *testing.T) {
		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		contributors, weeks, err := collectContributorStats(t.Context(), gitRepo, repoID, parentID)
		require.NoError(t, err)
		setStats(t, parentID, append(contributors, &repo_model.RepoContributor{RepoID: repoID, Email: marker.Email, Name: marker.Name, Commits: marker.Commits}), weeks)

		require.NoError(t, UpdateRepoContributors(t.Context(), repoID))
		assertStats(t, append([]*repo_model.RepoContributor{marker}, countedContributors...))
	})

	t.Run("RewrittenHistory", func(t *testing.T) {
		setStats(t, "0123456789012345678901234567890123456789", []*repo_model.RepoContributor{{RepoID: repoID, Email: marker.Email, Name: marker.Name, Commits: marker.Commits}}, nil)

		require.NoError(t, UpdateRepoContributors(t.Context(), repoID))
		assertStats(t, countedContributors)
	})
}
