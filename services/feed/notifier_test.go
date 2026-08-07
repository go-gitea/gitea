// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"strings"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID})
	repo.Owner = user

	oldRepoName := repo.Name
	const newRepoName = "newRepoName"
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionRenameRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().RenameRepository(t.Context(), user, repo, oldRepoName)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

func TestCommentActionExcerpts(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue.Repo = repo
	comment := &issues_model.Comment{
		Type: issues_model.CommentTypeComment, PosterID: doer.ID, IssueID: issue.ID,
		Content: "```suggestion\nreplacement\n```\n\nFirst meaningful **excerpt**\nSecond line",
	}
	assert.NoError(t, db.Insert(t.Context(), comment))

	NewNotifier().CreateIssueComment(t.Context(), doer, repo, issue, comment, nil)
	action := unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: doer.ID, UserID: doer.ID, RepoID: repo.ID,
		OpType: activities_model.ActionCommentIssue, CommentID: comment.ID,
	})
	assert.Equal(t, "1|First meaningful **excerpt**", action.Content)

	codeComment := &issues_model.Comment{
		Type: issues_model.CommentTypeCode, PosterID: doer.ID, IssueID: issue.ID,
		Content: "```suggestion\nreplacement\n```\n\nInline **excerpt**",
	}
	reviewComment := &issues_model.Comment{
		Type: issues_model.CommentTypeReview, PosterID: doer.ID, IssueID: issue.ID,
		Content: "\n\n# Review **excerpt**",
	}
	assert.NoError(t, db.Insert(t.Context(), codeComment, reviewComment))
	review := &issues_model.Review{
		Type: issues_model.ReviewTypeComment, Reviewer: doer, ReviewerID: doer.ID,
		Issue: issue, IssueID: issue.ID,
		CodeComments: issues_model.CodeComments{"file.txt": {1: {codeComment}}},
	}
	NewNotifier().PullRequestReview(t.Context(), nil, review, reviewComment, nil)
	inlineAction := unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: doer.ID, UserID: doer.ID, RepoID: repo.ID,
		OpType: activities_model.ActionCommentPull, CommentID: codeComment.ID,
	})
	reviewAction := unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: doer.ID, UserID: doer.ID, RepoID: repo.ID,
		OpType: activities_model.ActionCommentPull, CommentID: reviewComment.ID,
	})
	assert.Equal(t, "1|Inline **excerpt**", inlineAction.Content)
	assert.Equal(t, "1|# Review **excerpt**", reviewAction.Content)
}
