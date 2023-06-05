// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	now := time.Now().Unix()
	comment, err := issues_model.CreateComment(db.DefaultContext, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeComment,
		Doer:    doer,
		Repo:    repo,
		Issue:   issue,
		Content: "Hello",
	})
	assert.NoError(t, err)
	then := time.Now().Unix()

	assert.EqualValues(t, issues_model.CommentTypeComment, comment.Type)
	assert.EqualValues(t, "Hello", comment.Content)
	assert.EqualValues(t, issue.ID, comment.IssueID)
	assert.EqualValues(t, doer.ID, comment.PosterID)
	unittest.AssertInt64InRange(t, now, then, int64(comment.CreatedUnix))
	unittest.AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	updatedIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
	unittest.AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestFetchCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	res, err := issues_model.FetchCodeComments(db.DefaultContext, issue, user)
	assert.NoError(t, err)
	assert.Contains(t, res, "README.md")
	assert.Contains(t, res["README.md"], int64(4))
	assert.Len(t, res["README.md"][4], 1)
	assert.Equal(t, int64(4), res["README.md"][4][0].ID)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	res, err = issues_model.FetchCodeComments(db.DefaultContext, issue, user2)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestAsCommentType(t *testing.T) {
	assert.Equal(t, issues_model.CommentType(0), issues_model.CommentTypeComment)
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType(""))
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType("nonsense"))
	assert.Equal(t, issues_model.CommentTypeComment, issues_model.AsCommentType("comment"))
	assert.Equal(t, issues_model.CommentTypePRUnScheduledToAutoMerge, issues_model.AsCommentType("pull_cancel_scheduled_merge"))
}
