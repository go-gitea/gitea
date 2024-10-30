// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
)

// CommentList defines a list of comments
type CommentList []*Comment

// LoadPosters loads posters
func (comments CommentList) LoadPosters(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	posterIDs := container.FilterSlice(comments, func(c *Comment) (int64, bool) {
		return c.PosterID, c.Poster == nil && c.PosterID > 0
	})

	posterMaps, err := getPostersByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if comment.Poster == nil {
			comment.Poster = getPoster(comment.PosterID, posterMaps)
		}
	}
	return nil
}

// getConversationIDs returns all the conversation ids on this comment list which conversation hasn't been loaded
func (comments CommentList) getConversationIDs() []int64 {
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.ConversationID, comment.Conversation == nil
	})
}

// Conversations returns all the conversations of comments
func (comments CommentList) Conversations() ConversationList {
	conversations := make(map[int64]*Conversation, len(comments))
	for _, comment := range comments {
		if comment.Conversation != nil {
			if _, ok := conversations[comment.Conversation.ID]; !ok {
				conversations[comment.Conversation.ID] = comment.Conversation
			}
		}
	}

	conversationList := make([]*Conversation, 0, len(conversations))
	for _, conversation := range conversations {
		conversationList = append(conversationList, conversation)
	}
	return conversationList
}

// LoadConversations loads conversations of comments
func (comments CommentList) LoadConversations(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	conversationIDs := comments.getConversationIDs()
	conversations := make(map[int64]*Conversation, len(conversationIDs))
	left := len(conversationIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", conversationIDs[:limit]).
			Rows(new(Conversation))
		if err != nil {
			return err
		}

		for rows.Next() {
			var conversation Conversation
			err = rows.Scan(&conversation)
			if err != nil {
				rows.Close()
				return err
			}

			conversations[conversation.ID] = &conversation
		}
		_ = rows.Close()

		left -= limit
		conversationIDs = conversationIDs[limit:]
	}

	for _, comment := range comments {
		if comment.Conversation == nil {
			comment.Conversation = conversations[comment.ConversationID]
		}
	}
	return nil
}

// getAttachmentCommentIDs only return the comment ids which possibly has attachments
func (comments CommentList) getAttachmentCommentIDs() []int64 {
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.ID, comment.Type.HasAttachmentSupport()
	})
}

// LoadAttachmentsByConversation loads attachments by conversation id
func (comments CommentList) LoadAttachmentsByConversation(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	attachments := make([]*repo_model.Attachment, 0, len(comments)/2)
	if err := db.GetEngine(ctx).Where("conversation_id=? AND comment_id>0", comments[0].ConversationID).Find(&attachments); err != nil {
		return err
	}

	commentAttachmentsMap := make(map[int64][]*repo_model.Attachment, len(comments))
	for _, attach := range attachments {
		commentAttachmentsMap[attach.CommentID] = append(commentAttachmentsMap[attach.CommentID], attach)
	}

	for _, comment := range comments {
		comment.Attachments = commentAttachmentsMap[comment.ID]
	}
	return nil
}

// LoadAttachments loads attachments
func (comments CommentList) LoadAttachments(ctx context.Context) (err error) {
	if len(comments) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(comments))
	commentsIDs := comments.getAttachmentCommentIDs()
	left := len(commentsIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("comment_id", commentsIDs[:limit]).
			Rows(new(repo_model.Attachment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var attachment repo_model.Attachment
			err = rows.Scan(&attachment)
			if err != nil {
				_ = rows.Close()
				return err
			}
			attachments[attachment.CommentID] = append(attachments[attachment.CommentID], &attachment)
		}

		_ = rows.Close()
		left -= limit
		commentsIDs = commentsIDs[limit:]
	}

	for _, comment := range comments {
		comment.Attachments = attachments[comment.ID]
	}
	return nil
}

// LoadAttributes loads attributes of the comments, except for attachments and
// comments
func (comments CommentList) LoadAttributes(ctx context.Context) (err error) {
	if err = comments.LoadPosters(ctx); err != nil {
		return err
	}

	if err = comments.LoadAttachments(ctx); err != nil {
		return err
	}

	return comments.LoadConversations(ctx)
}
