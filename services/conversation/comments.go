// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversation

import (
	"context"

	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

// CreateConversationComment creates a plain conversation comment.
func CreateConversationComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, conversation *conversations_model.Conversation, content string, attachments []string) (*conversations_model.Comment, error) {
	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		if isAdmin, _ := access_model.IsUserRepoAdmin(ctx, repo, doer); !isAdmin {
			return nil, user_model.ErrBlockedUser
		}
	}

	comment, err := conversations_model.CreateComment(ctx, &conversations_model.CreateCommentOptions{
		Type:         conversations_model.CommentTypeComment,
		Doer:         doer,
		Repo:         repo,
		Conversation: conversation,
		Content:      content,
		Attachments:  attachments,
	})
	if err != nil {
		return nil, err
	}

	//notify_service.CreateConversationComment(ctx, doer, repo, conversation, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(ctx context.Context, c *conversations_model.Comment, contentVersion int, doer *user_model.User, oldContent string) error {
	if err := c.LoadConversation(ctx); err != nil {
		return err
	}
	if err := c.Conversation.LoadRepo(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, doer, c.Conversation.Repo.OwnerID) {
		if isAdmin, _ := access_model.IsUserRepoAdmin(ctx, c.Conversation.Repo, doer); !isAdmin {
			return user_model.ErrBlockedUser
		}
	}

	needsContentHistory := c.Content != oldContent && c.Type.HasContentSupport()
	if needsContentHistory {
		hasContentHistory, err := conversations_model.HasConversationContentHistory(ctx, c.ConversationID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = conversations_model.SaveConversationContentHistory(ctx, c.PosterID, c.ConversationID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := conversations_model.UpdateComment(ctx, c, contentVersion, doer); err != nil {
		return err
	}

	if needsContentHistory {
		err := conversations_model.SaveConversationContentHistory(ctx, doer.ID, c.ConversationID, c.ID, timeutil.TimeStampNow(), c.Content, false)
		if err != nil {
			return err
		}
	}

	//notify_service.UpdateComment(ctx, doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, doer *user_model.User, comment *conversations_model.Comment) error {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return conversations_model.DeleteComment(ctx, comment)
	})
	if err != nil {
		return err
	}

	//notify_service.DeleteComment(ctx, doer, comment)

	return nil
}
