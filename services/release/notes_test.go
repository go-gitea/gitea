// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateReleaseNotes(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("ChangeLogsWithPRs", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		t.Cleanup(func() { gitRepo.Close() })

		mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
		createMergedPullRequest(t, repo, mergedCommit, 5, "Release notes test pull request")

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
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		t.Cleanup(func() { gitRepo.Close() })

		createMergedPullRequest(t, repo, "69554a64c1e6030f051e5c3f94bfbd773cd6a324", 5, "Initial tag PR 1")
		createMergedPullRequest(t, repo, "27566bd5738fc8b4e3fef3c5e72cce608537bd95", 4, "Initial tag PR 2")
		createMergedPullRequest(t, repo, "5099b81332712fe655e34e8dd63574f503f61811", 8, "Initial tag PR 3")

		content, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
			TagName:   "v0.1.0",
			TagTarget: repo.DefaultBranch,
		})
		require.NoError(t, err)

		assert.Contains(t, content, "## What's Changed\n")
		assert.Contains(t, content, "* Initial tag PR 1 in [#")
		assert.Contains(t, content, "* Initial tag PR 2 in [#")
		assert.Contains(t, content, "* Initial tag PR 3 in [#")
		assert.Contains(t, content, "\n## Contributors\n")
		assert.Contains(t, content, "* @user5\n")
		assert.Contains(t, content, "* @user4\n")
		assert.Contains(t, content, "* @user8\n")
		assert.Contains(t, content, "\n## New Contributors\n")
		assert.Contains(t, content, "* @user5 made their first contribution in [#")
		assert.Contains(t, content, "* @user4 made their first contribution in [#")
		assert.Contains(t, content, "* @user8 made their first contribution in [#")
		assert.Contains(t, content, "**Full Changelog**: https://try.gitea.io/user2/repo16/commits/tag/v0.1.0\n")
	})

	t.Run("EmptyPreviousTagWithExistingTags", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		t.Cleanup(func() { gitRepo.Close() })

		_, err = GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
			TagName:   "v1.2.0",
			TagTarget: repo.DefaultBranch,
		})
		require.Error(t, err)
	})
}

func createMergedPullRequest(t *testing.T, repo *repo_model.Repository, mergeCommit string, posterID int64, title string) *issues_model.PullRequest {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: posterID})

	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Poster:   user,
		PosterID: user.ID,
		Title:    title,
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
