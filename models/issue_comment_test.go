// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateComment(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	issue := AssertExistsAndLoadBean(t, &Issue{}).(*Issue)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: issue.RepoID}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)

	now := time.Now().Unix()
	comment, err := CreateComment(&CreateCommentOptions{
		Type:    CommentTypeComment,
		Doer:    doer,
		Repo:    repo,
		Issue:   issue,
		Content: "Hello",
	})
	assert.NoError(t, err)
	then := time.Now().Unix()

	assert.EqualValues(t, CommentTypeComment, comment.Type)
	assert.EqualValues(t, "Hello", comment.Content)
	assert.EqualValues(t, issue.ID, comment.IssueID)
	assert.EqualValues(t, doer.ID, comment.PosterID)
	AssertInt64InRange(t, now, then, int64(comment.CreatedUnix))
	AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	updatedIssue := AssertExistsAndLoadBean(t, &Issue{ID: issue.ID}).(*Issue)
	AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestFetchCodeComments(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	issue := AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	res, err := FetchCodeComments(issue, user)
	assert.NoError(t, err)
	assert.Contains(t, res, "README.md")
	assert.Contains(t, res["README.md"], int64(4))
	assert.Len(t, res["README.md"][4], 1)
	assert.Equal(t, int64(4), res["README.md"][4][0].ID)

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	res, err = FetchCodeComments(issue, user2)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
}
