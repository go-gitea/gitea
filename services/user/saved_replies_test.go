// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavedReplyCRUD(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// create
	require.NoError(t, CreateSavedReply(t.Context(), user2, "greeting", "Hello, thanks for the report!"))
	require.NoError(t, CreateSavedReply(t.Context(), user2, "wontfix", "This is working as intended."))

	replies, err := user_model.GetUserSavedReplies(t.Context(), user2.ID, "")
	require.NoError(t, err)
	require.Len(t, replies, 2)
	assert.Equal(t, "greeting", replies[0].Title)
	assert.Equal(t, "wontfix", replies[1].Title)
	assert.Equal(t, "Hello, thanks for the report!", replies[0].Content)
	assert.Equal(t, "This is working as intended.", replies[1].Content)

	replyID := replies[0].ID

	// update
	require.NoError(t, UpdateSavedReply(t.Context(), user2, replyID, "updated", "Updated content"))
	updated, err := user_model.GetSavedReply(t.Context(), replyID)
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Title)
	assert.Equal(t, "Updated content", updated.Content)

	// delete
	require.NoError(t, DeleteSavedReply(t.Context(), user2, replyID))
	_, err = user_model.GetSavedReply(t.Context(), replyID)
	assert.Error(t, err)

	// remaining reply
	replies, err = user_model.GetUserSavedReplies(t.Context(), user2.ID, "")
	require.NoError(t, err)
	assert.Len(t, replies, 1)

	// user4 cannot see user2's replies
	otherReplies, err := user_model.GetUserSavedReplies(t.Context(), user4.ID, "")
	require.NoError(t, err)
	assert.Empty(t, otherReplies)
}

func TestSavedReplyValidation(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	err := CreateSavedReply(t.Context(), user2, "", "content")
	assert.ErrorIs(t, err, user_model.ErrSavedReplyTitleEmpty)

	err = CreateSavedReply(t.Context(), user2, "   ", "content")
	assert.ErrorIs(t, err, user_model.ErrSavedReplyTitleEmpty)

	err = CreateSavedReply(t.Context(), user2, "title", "")
	assert.ErrorIs(t, err, user_model.ErrSavedReplyContentEmpty)
}

func TestSavedReplyOwnership(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	require.NoError(t, CreateSavedReply(t.Context(), user2, "private", "user2 only"))

	replies, err := user_model.GetUserSavedReplies(t.Context(), user2.ID, "")
	require.NoError(t, err)
	require.NotEmpty(t, replies)
	replyID := replies[0].ID

	// user4 cannot update user2's reply
	err = UpdateSavedReply(t.Context(), user4, replyID, "hacked", "hacked")
	assert.ErrorIs(t, err, user_model.ErrSavedReplyDoesNotBelongToUser)

	// user4 cannot delete user2's reply
	err = DeleteSavedReply(t.Context(), user4, replyID)
	assert.ErrorIs(t, err, user_model.ErrSavedReplyDoesNotBelongToUser)

	// verify reply unchanged
	reply, err := user_model.GetSavedReply(t.Context(), replyID)
	require.NoError(t, err)
	assert.Equal(t, "private", reply.Title)

	// cleanup
	_, err = db.DeleteByID[user_model.SavedReply](t.Context(), replyID)
	require.NoError(t, err)
}
