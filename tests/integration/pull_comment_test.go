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
	issues_service "code.gitea.io/gitea/services/issue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWaitForPullRequestStatus(t *testing.T, prIssue *issues_model.Issue, expectedStatus issues_model.PullRequestStatus) (retIssue *issues_model.Issue) {
	require.Eventually(t, func() bool {
		prIssueCond := *prIssue
		retIssue = unittest.AssertExistsAndLoadBean(t, &prIssueCond)
		require.NoError(t, retIssue.LoadPullRequest(t.Context()))
		return retIssue.PullRequest.Status == expectedStatus
	}, 5*time.Second, 20*time.Millisecond)
	return retIssue
}

func testPullCommentRebase(t *testing.T, u *url.URL, session *TestSession) {
	testPRTitle := "Test PR for rebase comment"
	// make a change on forked branch
	testEditFile(t, session, "user1", "repo1", "test-branch/rebase", "README.md", "Hello, World (Edited)\n")
	testPullCreate(t, session, "user1", "repo1", false, "test-branch/rebase", "test-branch/rebase", testPRTitle)
	// create a conflict on base repo branch
	testEditFile(t, session, "user2", "repo1", "test-branch/rebase", "README.md", "Hello, World (Edited Conflicted)\n")

	// Now the pull request status should be conflicted
	testWaitForPullRequestStatus(t, &issues_model.Issue{Title: testPRTitle}, issues_model.PullRequestStatusConflict)

	dstPath := t.TempDir()
	u.Path = "/user2/repo1.git"
	doGitClone(dstPath, u)(t)
	doGitCheckoutBranch(dstPath, "test-branch/rebase")(t)
	doGitCreateBranch(dstPath, "local-branch/rebase")(t)
	content, _ := os.ReadFile(dstPath + "/README.md")
	require.Equal(t, "Hello, World (Edited Conflicted)\n", string(content))

	doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
		LocalRepoPath:   dstPath,
		CheckoutBranch:  "local-branch/rebase",
		TreeFilePath:    "README.md",
		TreeFileContent: "Hello, World (Edited Conflict Resolved)\n",
	})(t)

	// do force push
	u.Path = "/user1/repo1.git"
	u.User = url.UserPassword("user1", userPassword)
	doGitAddRemote(dstPath, "base-repo", u)(t)
	doGitPushTestRepositoryFail(dstPath, "base-repo", "local-branch/rebase:test-branch/rebase")(t)
	doGitPushTestRepository(dstPath, "--force", "base-repo", "local-branch/rebase:test-branch/rebase")(t)

	// reload the pr
	prIssue := testWaitForPullRequestStatus(t, &issues_model.Issue{Title: testPRTitle}, issues_model.PullRequestStatusMergeable)
	comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: prIssue.ID,
		Type:    issues_model.CommentTypeUndefined, // get all comments type
	})
	require.NoError(t, err)
	lastComment := comments[len(comments)-1]
	assert.NoError(t, issues_service.LoadCommentPushCommits(t.Context(), lastComment))
	assert.True(t, lastComment.IsForcePush)
}

func testPullCommentRetarget(t *testing.T, u *url.URL, session *TestSession) {
	testPRTitle := "Test PR for retarget comment"
	// keep a non-conflict branch
	testCreateBranch(t, session, "user2", "repo1", "branch/test-branch/retarget", "test-branch/retarget-no-conflict", http.StatusSeeOther)
	// make a change on forked branch
	testEditFile(t, session, "user1", "repo1", "test-branch/retarget", "README.md", "Hello, World (Edited)\n")
	testPullCreate(t, session, "user1", "repo1", false, "test-branch/retarget", "test-branch/retarget", testPRTitle)
	// create a conflict line on user2/repo1 README.md
	testEditFile(t, session, "user2", "repo1", "test-branch/retarget", "README.md", "Hello, World (Edited Conflicted)\n")

	// Now the pull request status should be conflicted
	prIssue := testWaitForPullRequestStatus(t, &issues_model.Issue{Title: testPRTitle}, issues_model.PullRequestStatusConflict)

	// do retarget
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/pull/%d/target_branch", prIssue.PullRequest.Index), map[string]string{
		"target_branch": "test-branch/retarget-no-conflict",
	})
	session.MakeRequest(t, req, http.StatusOK)
	testWaitForPullRequestStatus(t, &issues_model.Issue{Title: testPRTitle}, issues_model.PullRequestStatusMergeable)
}

func TestPullComment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testCreateBranch(t, session, "user2", "repo1", "branch/master", "test-branch/rebase", http.StatusSeeOther)
		testCreateBranch(t, session, "user2", "repo1", "branch/master", "test-branch/retarget", http.StatusSeeOther)
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")

		t.Run("RebaseComment", func(t *testing.T) { testPullCommentRebase(t, u, session) })
		t.Run("RetargetComment", func(t *testing.T) { testPullCommentRetarget(t, u, session) })
	})
}
