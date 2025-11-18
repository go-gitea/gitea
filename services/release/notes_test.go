// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateReleaseNotes(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	t.Cleanup(func() { gitRepo.Close() })

	mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
	pr := createMergedPullRequest(t, repo, mergedCommit, 5)

	result, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
		TagName: "v1.2.0",
		Target:  "DefaultBranch",
	})
	require.NoError(t, err)

	assert.Equal(t, "v1.1", result.PreviousTag)
	assert.Contains(t, result.Content, "## What's Changed")
	assert.Contains(t, result.Content, pr.Issue.Title)
	assert.Contains(t, result.Content, fmt.Sprintf("/pulls/%d", pr.Index))
	assert.Contains(t, result.Content, "## Contributors")
	assert.Contains(t, result.Content, "@user5")
	assert.Contains(t, result.Content, "## New Contributors")
	assert.Contains(t, result.Content, repo.HTMLURL(t.Context())+"/compare/v1.1...v1.2.0")
}

func TestGenerateReleaseNotes_NoReleaseFallsBackToTags(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	t.Cleanup(func() { gitRepo.Close() })

	mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
	createMergedPullRequest(t, repo, mergedCommit, 5)

	var releases []repo_model.Release
	err = db.GetEngine(t.Context()).
		Where("repo_id=?", repo.ID).
		Asc("id").
		Find(&releases)
	require.NoError(t, err)

	_, err = db.GetEngine(t.Context()).
		Where("repo_id=?", repo.ID).
		Delete(new(repo_model.Release))
	require.NoError(t, err)
	t.Cleanup(func() {
		if len(releases) == 0 {
			return
		}
		ctx := context.Background()
		_, err := db.GetEngine(ctx).Insert(&releases)
		require.NoError(t, err)
	})

	result, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
		TagName: "v1.2.0",
		Target:  "DefaultBranch",
	})
	require.NoError(t, err)
	assert.Equal(t, "v1.1", result.PreviousTag)
	assert.Contains(t, result.Content, "@user5")
}

func createMergedPullRequest(t *testing.T, repo *repo_model.Repository, mergeCommit string, posterID int64) *issues_model.PullRequest {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: posterID})

	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Poster:   user,
		PosterID: user.ID,
		Title:    "Release notes test pull request",
		Content:  "content",
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: repo.ID,
		BaseRepoID: repo.ID,
		HeadBranch: repo.DefaultBranch,
		BaseBranch: repo.DefaultBranch,
		Status:     issues_model.PullRequestStatusMergeable,
		Flow:       issues_model.PullRequestFlowGithub,
	}

	require.NoError(t, issues_model.NewPullRequest(t.Context(), repo, issue, nil, nil, pr))

	pr.HasMerged = true
	pr.MergedCommitID = mergeCommit
	pr.MergedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(t.Context()).
		ID(pr.ID).
		Cols("has_merged", "merged_commit_id", "merged_unix").
		Update(pr)
	require.NoError(t, err)

	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.Issue.LoadAttributes(t.Context()))
	return pr
}
