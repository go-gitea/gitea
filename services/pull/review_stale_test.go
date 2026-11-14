// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	pull_service "gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateCodeCommentWithStalePendingReview ensures a pending review from an
// earlier commit does not prevent commenting on lines added to the PR head.
func TestCreateCodeCommentWithStalePendingReview(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6})
	require.NoError(t, pr.LoadBaseRepo(ctx))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	require.NoError(t, issue.LoadRepo(ctx))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	const treePath = "stale-pending-review.txt"
	headRef := pr.GetGitHeadRefName()
	require.NoError(t, git.ForceFastImport(ctx, pr.BaseRepo, []git.FastImportCommit{
		{
			Ref:   headRef,
			Files: []git.FastImportFile{{Path: treePath, Content: "existing line\n"}},
		},
		{
			Ref:    headRef,
			Parent: ":1",
			Files:  []git.FastImportFile{{Path: treePath, Content: "existing line\nnew line\n"}},
		},
	}))

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	headCommit, err := gitRepo.GetCommit(ctx, headRef)
	require.NoError(t, err)
	oldCommitID, err := headCommit.ParentID(0)
	require.NoError(t, err)
	pr.MergeBase = oldCommitID.String()
	require.NoError(t, pr.UpdateCols(ctx, "merge_base"))

	pendingReview, err := issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypePending,
		Reviewer: doer,
		Issue:    issue,
		CommitID: oldCommitID.String(),
	})
	require.NoError(t, err)

	// Line 2 does not exist in the pending review's recorded commit.
	// Blame must fall back to the current PR head before creating the comment.
	comment, err := pull_service.CreateCodeComment(
		ctx, doer, gitRepo, issue, 2, "comment on new line", treePath,
		true, 0, headCommit.ID.String(), nil,
	)
	require.NoError(t, err)
	require.NotNil(t, comment)
	assert.Equal(t, pendingReview.ID, comment.ReviewID)
	assert.Equal(t, headCommit.ID.String(), comment.CommitSHA)
	assert.Contains(t, comment.Patch, "+new line")
}
