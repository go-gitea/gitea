// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"fmt"
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/util"
	pull_service "gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDismissReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Issue:    issue,
		Reviewer: reviewer,
		Type:     issues_model.ReviewTypeReject,
	})

	assert.NoError(t, err)
	for _, hasMerged := range []bool{false, true} {
		issue.IsClosed = !hasMerged
		pull.HasMerged = hasMerged
		assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))
		assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
		_, err = pull_service.DismissReview(t.Context(), review.ID, issue.RepoID, "", &user_model.User{}, false, false)
		assert.Error(t, err)
		assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))
	}
}

func TestCreateCodeCommentContextOfUnchangedLine(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6})
	require.NoError(t, pr.LoadBaseRepo(ctx))

	numberedLines := func(prefix string, count int) string {
		var sb strings.Builder
		for i := range count {
			_, _ = fmt.Fprintf(&sb, "%s %d\n", prefix, i+1)
		}
		return sb.String()
	}

	// "old 10" was last changed by the first commit (line 10), it is line 15 in the merge base and line 17 in the PR head
	const treePath = "unchanged-line.txt"
	oldLines := numberedLines("old", 20)
	baseLines := numberedLines("base", 5) + oldLines
	headLines := numberedLines("head", 2) + baseLines

	headRef := pr.GetGitHeadRefName()
	require.NoError(t, git.ForceFastImport(ctx, pr.BaseRepo, []git.FastImportCommit{
		{Ref: headRef, Files: []git.FastImportFile{{Path: treePath, Content: oldLines}}},
		{Ref: headRef, Parent: ":1", Files: []git.FastImportFile{{Path: treePath, Content: baseLines}}},
		{Ref: headRef, Parent: ":2", Files: []git.FastImportFile{{Path: treePath, Content: headLines}}},
	}))

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	headCommit, err := gitRepo.GetCommit(ctx, headRef)
	require.NoError(t, err)
	mergeBase, err := headCommit.ParentID(0)
	require.NoError(t, err)
	mergeBaseCommit, err := gitRepo.GetCommit(ctx, mergeBase.String())
	require.NoError(t, err)
	lineCommitID, err := mergeBaseCommit.ParentID(0)
	require.NoError(t, err)
	pr.MergeBase = mergeBase.String()
	require.NoError(t, pr.UpdateCols(ctx, "merge_base"))

	for _, c := range []struct {
		name     string
		line     int64
		doerID   int64
		commitID string
	}{
		{"NewSide", 17, 2, ""},
		{"OldSide", -15, 2, ""},
		{"UnknownReviewCommit", 17, 4, strings.Repeat("a", 40)},
	} {
		t.Run(c.name, func(t *testing.T) {
			doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: c.doerID})
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
			require.NoError(t, issue.LoadRepo(ctx))
			comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, c.line, "test", treePath, true, 0, c.commitID, nil)
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(comment.Patch, "\n old 10\n"), "patch: %q", comment.Patch)
			if c.line > 0 {
				assert.Equal(t, lineCommitID.String(), comment.CommitSHA)
				require.NoError(t, pull_service.InvalidateCodeComments(ctx, issues_model.PullRequestList{pr}, pr.BaseRepo, headRef))
				comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID})
				assert.False(t, comment.Invalidated)
			}
		})
	}
}

func TestCreateCodeCommentUnknownCommitID(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// org3/repo3.git: "test_branch" exists, but "refs/pull/2/head" (PR ID: 6) doesn't exist yet
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6})
	require.NoError(t, pr.LoadBaseRepo(ctx))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	require.NoError(t, issue.LoadRepo(ctx))

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	const treePath = "file.txt"

	t.Run("RollbackOnFailure", func(t *testing.T) {
		// the PR head ref doesn't exist yet, so creating the comment fails
		_, err := gitRepo.GetCommit(ctx, pr.GetGitHeadRefName())
		require.ErrorIs(t, err, util.ErrNotExist)
		_, err = pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, 1, "test", treePath, true, 0, "", nil)
		require.Error(t, err)
		_, err = issues_model.GetCurrentReview(ctx, doer, issue)
		assert.True(t, issues_model.IsErrReviewNotExist(err), "got: %v", err)
	})

	t.Run("FallbackToHead", func(t *testing.T) {
		headCommitID, err := gitRepo.GetBranchCommitID(ctx, pr.HeadBranch)
		require.NoError(t, err)
		require.NoError(t, git.UpdateRef(ctx, gitRepo, pr.GetGitHeadRefName(), headCommitID))
		// the first comment creates the pending review with the unknown commit, the second one reuses it
		for i, commitID := range []string{strings.Repeat("a", 40), headCommitID} {
			comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, int64(i+1), "test", treePath, true, 0, commitID, nil)
			require.NoError(t, err)
			assert.Equal(t, headCommitID, comment.CommitSHA)
		}
	})
}
