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
	comment, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeComment,
		Doer:    doer,
		Repo:    repo,
		Issue:   issue,
		Content: "Hello",
	})
	assert.NoError(t, err)
	then := time.Now().Unix()

	assert.Equal(t, issues_model.CommentTypeComment, comment.Type)
	assert.Equal(t, "Hello", comment.Content)
	assert.Equal(t, issue.ID, comment.IssueID)
	assert.Equal(t, doer.ID, comment.PosterID)
	unittest.AssertInt64InRange(t, now, then, int64(comment.CreatedUnix))
	unittest.AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	updatedIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
	unittest.AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func Test_UpdateCommentAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 1})
	attachment := repo_model.Attachment{
		Name: "test.txt",
	}
	assert.NoError(t, db.Insert(t.Context(), &attachment))

	err := issues_model.UpdateCommentAttachments(t.Context(), comment, []string{attachment.UUID})
	assert.NoError(t, err)

	attachment2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: attachment.ID})
	assert.Equal(t, attachment.Name, attachment2.Name)
	assert.Equal(t, comment.ID, attachment2.CommentID)
	assert.Equal(t, comment.IssueID, attachment2.IssueID)
}

func TestFetchCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	res, err := issues_model.FetchCodeComments(t.Context(), issue, user, false)
	assert.NoError(t, err)
	assert.Contains(t, res, "README.md")
	assert.Contains(t, res["README.md"], int64(4))
	assert.Len(t, res["README.md"][4], 1)
	assert.Equal(t, int64(4), res["README.md"][4][0].ID)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	res, err = issues_model.FetchCodeComments(t.Context(), issue, user2, false)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestAsCommentType(t *testing.T) {
	assert.Equal(t, issues_model.CommentTypeComment, issues_model.CommentType(0))
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType(""))
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType("nonsense"))
	assert.Equal(t, issues_model.CommentTypeComment, issues_model.AsCommentType("comment"))
	assert.Equal(t, issues_model.CommentTypePRUnScheduledToAutoMerge, issues_model.AsCommentType("pull_cancel_scheduled_merge"))
}

func TestMigrate_InsertIssueComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	_ = issue.LoadRepo(t.Context())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})
	reaction := &issues_model.Reaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	comment := &issues_model.Comment{
		PosterID:  owner.ID,
		Poster:    owner,
		IssueID:   issue.ID,
		Issue:     issue,
		Reactions: []*issues_model.Reaction{reaction},
	}

	err := issues_model.InsertIssueComments(t.Context(), []*issues_model.Comment{comment})
	assert.NoError(t, err)

	issueModified := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.Equal(t, issue.NumComments+1, issueModified.NumComments)

	unittest.CheckConsistencyFor(t, &issues_model.Issue{})
}

func Test_UpdateIssueNumComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	assert.NoError(t, issues_model.UpdateIssueNumComments(t.Context(), issue2.ID))
	issue2 = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	assert.Equal(t, 1, issue2.NumComments)
}
