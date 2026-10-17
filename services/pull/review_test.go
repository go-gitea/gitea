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
	issue.IsClosed = true
	pull.HasMerged = false
	assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	_, err = pull_service.DismissReview(t.Context(), review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))

	pull.HasMerged = true
	pull.Issue.IsClosed = false
	assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	_, err = pull_service.DismissReview(t.Context(), review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))
}

func TestCreateCodeCommentContextOfUnchangedLine(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
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
	pr.MergeBase = mergeBase.String()
	require.NoError(t, pr.UpdateCols(ctx, "merge_base"))

	for _, c := range []struct {
		name string
		line int64
	}{
		{"NewSide", 17},
		{"OldSide", -15},
	} {
		t.Run(c.name, func(t *testing.T) {
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
			require.NoError(t, issue.LoadRepo(ctx))
			comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, c.line, "test", treePath, true, 0, headCommit.ID.String(), nil)
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(comment.Patch, "\n old 10\n"), "patch: %q", comment.Patch)
		})
	}
}
