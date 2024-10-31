// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversation

import (
	"context"

	conversations_model "code.gitea.io/gitea/models/conversations"
	user_model "code.gitea.io/gitea/models/user"
)

// CreateCommentReaction creates a reaction on a comment.
func CreateCommentReaction(ctx context.Context, doer *user_model.User, comment *conversations_model.ConversationComment, content string) (*conversations_model.CommentReaction, error) {
	if err := comment.LoadConversation(ctx); err != nil {
		return nil, err
	}

	if err := comment.Conversation.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if user_model.IsUserBlockedBy(ctx, doer, comment.Conversation.Repo.OwnerID, comment.PosterID) {
		return nil, user_model.ErrBlockedUser
	}

	return conversations_model.CreateReaction(ctx, &conversations_model.ReactionOptions{
		Type:           content,
		DoerID:         doer.ID,
		ConversationID: comment.Conversation.ID,
		CommentID:      comment.ID,
	})
}
