// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
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

	t.Run("ChangeLogsWithPRs", func(t *testing.T) {
		mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
		createMergedPullRequest(t, repo, mergedCommit, 5)

		content, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
			TagName:     "v1.2.0",
			TagTarget:   "DefaultBranch",
			PreviousTag: "v1.1",
		})
		require.NoError(t, err)

		assert.Equal(t, `## What's Changed
* Release notes test pull request in [#6](https://try.gitea.io/user2/repo1/pulls/6)

## Contributors
* @user5

## New Contributors
* @user5 made their first contribution in [#6](https://try.gitea.io/user2/repo1/pulls/6)

**Full Changelog**: [v1.1...v1.2.0](https://try.gitea.io/user2/repo1/compare/v1.1...v1.2.0)
`, content)
	})

	t.Run("NoPreviousTag", func(t *testing.T) {
		content, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
			TagName:   "v1.2.0",
			TagTarget: "DefaultBranch",
		})
		require.NoError(t, err)
		assert.Equal(t, "**Full Changelog**: https://try.gitea.io/user2/repo1/commits/tag/v1.2.0\n", content)
	})
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
