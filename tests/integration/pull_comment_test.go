// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git/gitcmd"
	issues_service "code.gitea.io/gitea/services/issue"

	"github.com/stretchr/testify/assert"
)

func TestPull_RebaseComment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		// create a conflict line on user2/repo1:master README.md
		testEditFile(t, session, "user2", "repo1", "master", "README.md", "Hello, World (Edited Conflicted)\n")

		// Now the pull request status should be conflicted
		prIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "This is a pull title"})
		assert.NoError(t, prIssue.LoadPullRequest(t.Context()))
		assert.Equal(t, issues_model.PullRequestStatusConflict, prIssue.PullRequest.Status)
		assert.NoError(t, prIssue.PullRequest.LoadBaseRepo(t.Context()))
		assert.Equal(t, "user2/repo1", prIssue.PullRequest.BaseRepo.FullName())
		assert.NoError(t, prIssue.PullRequest.LoadHeadRepo(t.Context()))
		assert.Equal(t, "user1/repo1", prIssue.PullRequest.HeadRepo.FullName())

		dstPath := t.TempDir()
		u.Path = "/user2/repo1.git"
		doGitClone(dstPath, u)(t)
		doGitCreateBranch(dstPath, "dev")(t)
		content, err := os.ReadFile(dstPath + "/README.md")
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World (Edited Conflicted)\n", string(content))

		doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
			LocalRepoPath:   dstPath,
			CheckoutBranch:  "dev",
			TreeFilePath:    "README.md",
			TreeFileContent: "Hello, World (Edited Conflict Resolved)\n",
		})(t)

		// do force push
		u.Path = "/user1/repo1.git"
		u.User = url.UserPassword("user1", userPassword)
		doGitAddRemote(dstPath, "fork", u)(t)
		// non force push will fail
		_, _, err = gitcmd.NewCommand().AddArguments("push", "fork", "dev:master").RunStdString(t.Context(), &gitcmd.RunOpts{Dir: dstPath})
		assert.Error(t, err)
		_, _, err = gitcmd.NewCommand().AddArguments("push", "--force", "fork", "dev:master").RunStdString(t.Context(), &gitcmd.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		time.Sleep(time.Second) // wait for pull request conflict checking

		// reload the pr
		prIssue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "This is a pull title"})
		assert.NoError(t, prIssue.LoadPullRequest(t.Context()))
		assert.Equal(t, issues_model.PullRequestStatusMergeable, prIssue.PullRequest.Status)
		comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: prIssue.ID,
			Type:    issues_model.CommentTypeUndefined, // get all comments type
		})
		assert.NoError(t, err)
		lastComment := comments[len(comments)-1]
		err = issues_service.LoadCommentPushCommits(t.Context(), lastComment)
		assert.NoError(t, err)
		assert.True(t, lastComment.IsForcePush)
	})
}

func TestPull_RetargetComment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		session2 := loginUser(t, "user2")
		// create a non-conflict branch dev from master
		testCreateBranch(t, session2, "user2", "repo1", "branch/master", "dev", http.StatusSeeOther)

		// create a conflict line on user2/repo1:master README.md
		testEditFile(t, session2, "user2", "repo1", "master", "README.md", "Hello, World (Edited Conflicted)\n")

		// Now the pull request status should be conflicted
		prIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "This is a pull title"})
		assert.NoError(t, prIssue.LoadPullRequest(t.Context()))
		assert.Equal(t, issues_model.PullRequestStatusConflict, prIssue.PullRequest.Status)
		assert.NoError(t, prIssue.PullRequest.LoadBaseRepo(t.Context()))
		assert.Equal(t, "user2/repo1", prIssue.PullRequest.BaseRepo.FullName())
		assert.NoError(t, prIssue.PullRequest.LoadHeadRepo(t.Context()))
		assert.Equal(t, "user1/repo1", prIssue.PullRequest.HeadRepo.FullName())

		// do retarget
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/pull/%d/target_branch", prIssue.PullRequest.Index), map[string]string{
			"_csrf":         GetUserCSRFToken(t, session2),
			"target_branch": "dev",
		})
		session2.MakeRequest(t, req, http.StatusOK)

		time.Sleep(time.Second) // wait for pull request conflict checking

		// reload the pr
		prIssue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "This is a pull title"})
		assert.NoError(t, prIssue.LoadPullRequest(t.Context()))
		assert.Equal(t, issues_model.PullRequestStatusMergeable, prIssue.PullRequest.Status)
	})
}
