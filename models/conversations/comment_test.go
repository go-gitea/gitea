// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations_test

import (
	"testing"
	"time"

	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	conversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: conversation.RepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	now := time.Now().Unix()
	comment, err := conversations_model.CreateComment(db.DefaultContext, &conversations_model.CreateCommentOptions{
		Type:         conversations_model.CommentTypeComment,
		Doer:         doer,
		Repo:         repo,
		Conversation: conversation,
		Content:      "Hello",
	})
	assert.NoError(t, err)
	then := time.Now().Unix()

	assert.EqualValues(t, conversations_model.CommentTypeComment, comment.Type)
	assert.EqualValues(t, "Hello", comment.Content)
	assert.EqualValues(t, conversation.ID, comment.ConversationID)
	assert.EqualValues(t, doer.ID, comment.PosterID)
	unittest.AssertInt64InRange(t, now, then, int64(comment.CreatedUnix))
	unittest.AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	updatedConversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: conversation.ID})
	unittest.AssertInt64InRange(t, now, then, int64(updatedConversation.UpdatedUnix))
}

func TestAsCommentType(t *testing.T) {
	assert.Equal(t, conversations_model.CommentType(0), conversations_model.CommentTypeComment)
	assert.Equal(t, conversations_model.CommentTypeUndefined, conversations_model.AsCommentType(""))
	assert.Equal(t, conversations_model.CommentTypeUndefined, conversations_model.AsCommentType("nonsense"))
	assert.Equal(t, conversations_model.CommentTypeComment, conversations_model.AsCommentType("comment"))
}

func TestMigrate_InsertConversationComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	conversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: 1})
	_ = conversation.LoadRepo(db.DefaultContext)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: conversation.Repo.OwnerID})
	reaction := &conversations_model.CommentReaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	comment := &conversations_model.ConversationComment{
		PosterID:       owner.ID,
		Poster:         owner,
		ConversationID: conversation.ID,
		Conversation:   conversation,
		Reactions:      []*conversations_model.CommentReaction{reaction},
	}

	err := conversations_model.InsertConversationComments(db.DefaultContext, []*conversations_model.ConversationComment{comment})
	assert.NoError(t, err)

	conversationModified := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: 1})
	assert.EqualValues(t, conversation.NumComments+1, conversationModified.NumComments)

	unittest.CheckConsistencyFor(t, &conversations_model.Conversation{})
}
