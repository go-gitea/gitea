// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadOpenNonPRIssue returns issue ID=1 (repo_id=1, is_closed=false, is_pull=false) from fixtures.
func loadOpenNonPRIssue(t *testing.T) *issues_model.Issue {
	t.Helper()
	issue, err := issues_model.GetIssueByID(t.Context(), 1)
	require.NoError(t, err)
	require.False(t, issue.IsClosed)
	require.False(t, issue.IsPull)
	return issue
}

// loadRepoOwnerDoer returns user_id=2 (owner of repo_id=1) from fixtures.
func loadRepoOwnerDoer(t *testing.T) *user_model.User {
	t.Helper()
	u, err := user_model.GetUserByID(t.Context(), 2)
	require.NoError(t, err)
	return u
}

func TestCloseIssue_Completed(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)

	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsCompleted()))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonCompleted, reloaded.CloseReason)
	assert.Empty(t, reloaded.CloseReasonParam)

	// The close comment must use CommentTypeCloseWithReason and carry the reason in its metadata.
	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonCompleted, cmt.CommentMetaData.CloseReason)
}

func TestCloseIssue_NotPlanned(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)

	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsNotPlanned()))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonNotPlanned, reloaded.CloseReason)
	assert.Empty(t, reloaded.CloseReasonParam)

	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonNotPlanned, cmt.CommentMetaData.CloseReason)
}

func TestCloseIssue_Duplicate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)

	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsDuplicate(4)))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonDuplicate, reloaded.CloseReason)
	assert.Contains(t, reloaded.CloseReasonParam, `"issue_index":4`)

	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonDuplicate, cmt.CommentMetaData.CloseReason)
	assert.Contains(t, cmt.CommentMetaData.CloseReasonParam, `"issue_index":4`)
}

func TestCloseIssue_Answered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)

	// comment id=2 in fixtures belongs to issue id=1.
	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsAnswered(2)))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonAnswered, reloaded.CloseReason)
	assert.Contains(t, reloaded.CloseReasonParam, `"comment_id":2`)

	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonAnswered, cmt.CommentMetaData.CloseReason)
	assert.Contains(t, cmt.CommentMetaData.CloseReasonParam, `"comment_id":2`)
}

func TestCloseIssue_CompletedByCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)
	commitHash := "deadbeef1234abcd"

	require.NoError(t, CloseIssue(t.Context(), issue, doer, commitHash, CloseOptionsCompletedByCommit(commitHash)))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonCompletedByCommit, reloaded.CloseReason)
	assert.Contains(t, reloaded.CloseReasonParam, commitHash)

	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonCompletedByCommit, cmt.CommentMetaData.CloseReason)
	assert.Contains(t, cmt.CommentMetaData.CloseReasonParam, commitHash)
}

func TestCloseIssue_CompletedByPull(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)
	const pullIndex = int64(42)

	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsCompletedByPull(pullIndex)))

	reloaded, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	assert.Equal(t, CloseReasonCompletedByPull, reloaded.CloseReason)
	assert.Contains(t, reloaded.CloseReasonParam, "42")

	cmt := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
	require.NotNil(t, cmt.CommentMetaData)
	assert.Equal(t, CloseReasonCompletedByPull, cmt.CommentMetaData.CloseReason)
	assert.Contains(t, cmt.CommentMetaData.CloseReasonParam, "42")
}

// TestReopenIssue_ClearsCloseReason verifies that reopening an issue clears its
// close reason and param so reopened issues never leak stale reason data.
func TestReopenIssue_ClearsCloseReason(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := loadOpenNonPRIssue(t)
	doer := loadRepoOwnerDoer(t)

	// Close with a reason first.
	require.NoError(t, CloseIssue(t.Context(), issue, doer, "", CloseOptionsNotPlanned()))
	closed, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	require.True(t, closed.IsClosed)
	require.Equal(t, CloseReasonNotPlanned, closed.CloseReason)

	// Reopen — reason must be cleared in the database.
	require.NoError(t, ReopenIssue(t.Context(), closed, doer, ""))

	reopened, err := issues_model.GetIssueByID(t.Context(), issue.ID)
	require.NoError(t, err)
	assert.False(t, reopened.IsClosed)
	assert.Empty(t, reopened.CloseReason)
	assert.Empty(t, reopened.CloseReasonParam)
}

// TestCloseIssue_PullRequest_NoReasonStored verifies that pull requests are
// excluded from the close-reason feature: even if a reason is passed, it must
// not be stored, and the close comment must be plain CommentTypeClose (not the
// new CommentTypeCloseWithReason).
func TestCloseIssue_PullRequest_NoReasonStored(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	// Issue ID=2 in repo_id=1 is a pull request (is_pull=true).
	pr, err := issues_model.GetIssueByID(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, pr.IsPull)
	require.False(t, pr.IsClosed)

	doer := loadRepoOwnerDoer(t)

	// Pass a reason — the service layer should strip it for PRs.
	require.NoError(t, CloseIssue(t.Context(), pr, doer, "", CloseOptionsNotPlanned()))

	reloaded, err := issues_model.GetIssueByID(t.Context(), pr.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsClosed)
	// PR must have no close reason stored.
	assert.Empty(t, reloaded.CloseReason)
	assert.Empty(t, reloaded.CloseReasonParam)

	// There must be NO CommentTypeCloseWithReason comment for this PR.
	unittest.AssertNotExistsBean(t, &issues_model.Comment{
		IssueID: pr.ID,
		Type:    issues_model.CommentTypeCloseWithReason,
	})
}
