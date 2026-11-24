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

func TestValidateCodeCommentRange(t *testing.T) {
	for _, test := range []struct {
		name       string
		start, end int64
		want       error
	}{
		{"OmittedStart", 0, 5, nil},
		{"SameEndpoint", 5, 5, nil},
		{"NewToOld", 5, -8, util.ErrInvalidArgument},
		{"ReversedNewSide", 8, 5, util.ErrInvalidArgument},
		{"ReversedOldSide", -8, -5, util.ErrInvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.ErrorIs(t, pull_service.ValidateCodeCommentRange(test.start, test.end), test.want)
		})
	}
}

func TestCreateCodeCommentOnUnchangedLines(t *testing.T) {
	unittest.PrepareTestEnv(t)
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
	lineCommit, err := gitRepo.GetCommit(ctx, headRef+"^^")
	require.NoError(t, err)
	pr.MergeBase = mergeBase.String()
	require.NoError(t, pr.UpdateCols(ctx, "merge_base"))

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherReviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	makeComment := func(doer *user_model.User, pending bool, start, end, reviewID int64) *issues_model.Comment {
		t.Helper()
		comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, pending, "", issues_model.CreateCommentOptions{
			LineNum: end, CodeStartLine: start, TreePath: treePath, Content: "test", ReviewID: reviewID,
		})
		require.NoError(t, err)
		return comment
	}

	single := makeComment(reviewer, true, 0, 17, 0)
	assert.Equal(t, lineCommit.ID.String(), single.CommitSHA)
	assert.Equal(t, headCommit.ID.String(), single.CodeCommitID())
	assert.True(t, strings.HasSuffix(single.Patch, "\n old 10\n"), "patch: %q", single.Patch)
	assert.True(t, strings.HasSuffix(makeComment(reviewer, true, 0, -15, 0).Patch, "\n old 10\n"))
	_, err = pull_service.CreateCodeComment(ctx, otherReviewer, gitRepo, issue, false, "", issues_model.CreateCommentOptions{
		LineNum: single.Line, Content: "reply", TreePath: treePath, ReviewID: single.ReviewID,
	})
	assert.True(t, issues_model.IsErrReviewNotExist(err), "got: %v", err)

	sameSide := makeComment(reviewer, true, 15, single.Line, 0)
	mixed := makeComment(reviewer, true, -2, single.Line, 0)
	const pushedRef = "refs/heads/range-invalidation"
	require.NoError(t, git.ForceFastImport(ctx, pr.BaseRepo, []git.FastImportCommit{{
		Ref: pushedRef, Parent: headCommit.ID.String(),
		Files: []git.FastImportFile{{Path: treePath, Content: strings.Replace(headLines, "old 9\n", "changed\n", 1)}},
	}}))
	require.NoError(t, pull_service.InvalidateCodeComments(ctx, issues_model.PullRequestList{pr}, pr.BaseRepo, pushedRef))
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: single.ID}).Invalidated)
	for _, comment := range []*issues_model.Comment{sameSide, mixed} {
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID}).Invalidated)
	}

	root := makeComment(otherReviewer, false, -9, 11, 0)
	require.NoError(t, git.UpdateRef(ctx, gitRepo, headRef, mergeBase.String()))
	reply := makeComment(otherReviewer, false, 0, root.Line, root.ReviewID)
	assert.Equal(t, headCommit.ID.String(), reply.CodeCommitID())
	assert.EqualValues(t, -9, reply.CodeStartLine())
	assert.EqualValues(t, 10, makeComment(otherReviewer, false, 10, root.Line, root.ReviewID).CodeStartLine())
}

func TestCreateCodeCommentUnknownCommitID(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6})
	require.NoError(t, pr.LoadBaseRepo(ctx))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	require.NoError(t, issue.LoadRepo(ctx))

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	options := issues_model.CreateCommentOptions{LineNum: 1, Content: "test", TreePath: "file.txt"}
	_, err = gitRepo.GetCommit(ctx, pr.GetGitHeadRefName())
	require.ErrorIs(t, err, util.ErrNotExist)
	_, err = pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, true, "", options)
	require.Error(t, err)
	_, err = issues_model.GetCurrentReview(ctx, doer, issue)
	assert.True(t, issues_model.IsErrReviewNotExist(err), "got: %v", err)

	headCommitID, err := gitRepo.GetBranchCommitID(ctx, pr.HeadBranch)
	require.NoError(t, err)
	require.NoError(t, git.UpdateRef(ctx, gitRepo, pr.GetGitHeadRefName(), headCommitID))
	for index, commitID := range []string{strings.Repeat("a", 40), headCommitID} {
		options.LineNum = int64(index + 1)
		comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, issue, true, commitID, options)
		require.NoError(t, err)
		assert.Equal(t, headCommitID, comment.CommitSHA)
		assert.Equal(t, headCommitID, comment.CodeCommitID())
	}
}
