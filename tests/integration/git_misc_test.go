// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/url"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func TestDataAsyncDoubleRead_Issue29101(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		testContent := bytes.Repeat([]byte{'a'}, 10000)
		resp, err := files_service.ChangeRepoFiles(t.Context(), repo, user, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "test.txt",
					ContentReader: bytes.NewReader(testContent),
				},
			},
			OldBranch: repo.DefaultBranch,
			NewBranch: repo.DefaultBranch,
		})
		assert.NoError(t, err)

		sha := resp.Commit.SHA

		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		assert.NoError(t, err)

		commit, err := gitRepo.GetCommit(sha)
		assert.NoError(t, err)

		entry, err := commit.GetTreeEntryByPath("test.txt")
		assert.NoError(t, err)

		b := entry.Blob()
		r1, err := b.DataAsync()
		assert.NoError(t, err)
		defer r1.Close()
		r2, err := b.DataAsync()
		assert.NoError(t, err)
		defer r2.Close()

		var data1, data2 []byte
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			data1, _ = io.ReadAll(r1)
			assert.NoError(t, err)
			wg.Done()
		}()
		go func() {
			data2, _ = io.ReadAll(r2)
			assert.NoError(t, err)
			wg.Done()
		}()
		wg.Wait()
		assert.Equal(t, testContent, data1)
		assert.Equal(t, testContent, data2)
	})
}

func TestAgitPullPush(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		baseAPITestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		u.Path = baseAPITestContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		dstPath := t.TempDir()
		doGitClone(dstPath, u)(t)

		gitRepo, err := git.OpenRepository(t.Context(), dstPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		doGitCreateBranch(dstPath, "test-agit-push")

		// commit 1
		_, err = generateCommitWithNewData(t.Context(), testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
		assert.NoError(t, err)

		// push to create an agit pull request
		err = git.NewCommand("push", "origin",
			"-o", "title=test-title", "-o", "description=test-description",
			"HEAD:refs/for/master/test-agit-push",
		).Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// check pull request exist
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: 1, Flow: issues_model.PullRequestFlowAGit, HeadBranch: "user2/test-agit-push"})
		assert.NoError(t, pr.LoadIssue(t.Context()))
		assert.Equal(t, "test-title", pr.Issue.Title)
		assert.Equal(t, "test-description", pr.Issue.Content)

		// commit 2
		_, err = generateCommitWithNewData(t.Context(), testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-2-")
		assert.NoError(t, err)

		// push 2
		err = git.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-push").Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// reset to first commit
		err = git.NewCommand("reset", "--hard", "HEAD~1").Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// test force push without confirm
		_, stderr, err := git.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-push").RunStdString(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.Error(t, err)
		assert.Contains(t, stderr, "[remote rejected] HEAD -> refs/for/master/test-agit-push (request `force-push` push option)")

		// test force push with confirm
		err = git.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-push", "-o", "force-push").Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	})
}

func TestAgitReviewStaleness(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		baseAPITestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		u.Path = baseAPITestContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		dstPath := t.TempDir()
		doGitClone(dstPath, u)(t)

		gitRepo, err := git.OpenRepository(t.Context(), dstPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		doGitCreateBranch(dstPath, "test-agit-review")

		// Create initial commit
		_, err = generateCommitWithNewData(t.Context(), testFileSizeSmall, dstPath, "user2@example.com", "User Two", "initial-")
		assert.NoError(t, err)

		// create PR via agit
		err = git.NewCommand("push", "origin",
			"-o", "title=Test agit Review Staleness", "-o", "description=Testing review staleness",
			"HEAD:refs/for/master/test-agit-review",
		).Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: 1,
			Flow:       issues_model.PullRequestFlowAGit,
			HeadBranch: "user2/test-agit-review",
		})
		assert.NoError(t, pr.LoadIssue(t.Context()))

		// Get initial commit ID for the review
		initialCommitID := pr.HeadCommitID
		t.Logf("Initial commit ID: %s", initialCommitID)

		// Create a review on the PR (as user1 reviewing user2's PR)
		reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
			Type:     issues_model.ReviewTypeApprove,
			Reviewer: reviewer,
			Issue:    pr.Issue,
			CommitID: initialCommitID,
			Content:  "LGTM! Looks good to merge.",
			Official: false,
		})
		assert.NoError(t, err)
		assert.False(t, review.Stale, "New review should not be stale")

		// Verify review exists and is not stale
		reviews, err := issues_model.FindReviews(t.Context(), issues_model.FindReviewOptions{
			IssueID: pr.IssueID,
		})
		assert.NoError(t, err)
		assert.Len(t, reviews, 1)
		assert.Equal(t, initialCommitID, reviews[0].CommitID)
		assert.False(t, reviews[0].Stale, "Review should not be stale initially")

		// Create a new commit and update the agit PR
		_, err = generateCommitWithNewData(t.Context(), testFileSizeSmall, dstPath, "user2@example.com", "User Two", "updated-")
		assert.NoError(t, err)

		err = git.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-review").Run(t.Context(), &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// Reload PR to get updated commit ID
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: 1,
			Flow:       issues_model.PullRequestFlowAGit,
			HeadBranch: "user2/test-agit-review",
		})
		assert.NoError(t, pr.LoadIssue(t.Context()))

		// For AGit PRs, HeadCommitID must be loaded from git references
		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		baseGitRepo, err := gitrepo.OpenRepository(t.Context(), baseRepo)
		assert.NoError(t, err)
		defer baseGitRepo.Close()

		updatedCommitID, err := baseGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
		assert.NoError(t, err)
		t.Logf("Updated commit ID: %s", updatedCommitID)

		// Verify the PR was updated with new commit
		assert.NotEqual(t, initialCommitID, updatedCommitID, "PR should have new commit ID after update")

		// Check that the review is now marked as stale
		reviews, err = issues_model.FindReviews(t.Context(), issues_model.FindReviewOptions{
			IssueID: pr.IssueID,
		})
		assert.NoError(t, err)
		assert.Len(t, reviews, 1)

		assert.True(t, reviews[0].Stale, "Review should be marked as stale after AGit PR update")

		// The review commit ID should remain the same (pointing to the original commit)
		assert.Equal(t, initialCommitID, reviews[0].CommitID, "Review commit ID should remain unchanged and point to original commit")
	})
}
