// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

// ConversationList defines a list of conversations
type ConversationList []*Conversation

// get the repo IDs to be loaded later, these IDs are for conversation.Repo and conversation.PullRequest.HeadRepo
func (conversations ConversationList) getRepoIDs() []int64 {
	return container.FilterSlice(conversations, func(conversation *Conversation) (int64, bool) {
		if conversation.Repo == nil {
			return conversation.RepoID, true
		}
		return 0, false
	})
}

// LoadRepositories loads conversations' all repositories
func (conversations ConversationList) LoadRepositories(ctx context.Context) (repo_model.RepositoryList, error) {
	if len(conversations) == 0 {
		return nil, nil
	}

	repoIDs := conversations.getRepoIDs()
	repoMaps := make(map[int64]*repo_model.Repository, len(repoIDs))
	left := len(repoIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", repoIDs[:limit]).
			Find(&repoMaps)
		if err != nil {
			return nil, fmt.Errorf("find repository: %w", err)
		}
		left -= limit
		repoIDs = repoIDs[limit:]
	}

	for _, conversation := range conversations {
		if conversation.Repo == nil {
			conversation.Repo = repoMaps[conversation.RepoID]
		} else {
			repoMaps[conversation.RepoID] = conversation.Repo
		}
	}
	return repo_model.ValuesRepository(repoMaps), nil
}

func (conversations ConversationList) getConversationIDs() []int64 {
	ids := make([]int64, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	return ids
}

// LoadAttachments loads attachments
func (conversations ConversationList) LoadAttachments(ctx context.Context) (err error) {
	if len(conversations) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(conversations))
	conversationsIDs := conversations.getConversationIDs()
	left := len(conversationsIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("conversation_id", conversationsIDs[:limit]).
			Rows(new(repo_model.Attachment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var attachment repo_model.Attachment
			err = rows.Scan(&attachment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("ConversationList.loadAttachments: Close: %w", err1)
				}
				return err
			}
			attachments[attachment.ConversationID] = append(attachments[attachment.ConversationID], &attachment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("ConversationList.loadAttachments: Close: %w", err1)
		}
		left -= limit
		conversationsIDs = conversationsIDs[limit:]
	}
	return nil
}

func (conversations ConversationList) loadComments(ctx context.Context, cond builder.Cond) (err error) {
	if len(conversations) == 0 {
		return nil
	}

	comments := make(map[int64][]*ConversationComment, len(conversations))
	conversationsIDs := conversations.getConversationIDs()
	left := len(conversationsIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).Table("conversation_comment").
			Join("INNER", "conversation", "conversation.id = conversation_comment.conversation_id").
			In("conversation.id", conversationsIDs[:limit]).
			Where(cond).
			NoAutoCondition().
			Rows(new(ConversationComment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var comment ConversationComment
			err = rows.Scan(&comment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("ConversationList.loadComments: Close: %w", err1)
				}
				return err
			}
			comments[comment.ConversationID] = append(comments[comment.ConversationID], &comment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("ConversationList.loadComments: Close: %w", err1)
		}
		left -= limit
		conversationsIDs = conversationsIDs[limit:]
	}

	for _, conversation := range conversations {
		conversation.Comments = comments[conversation.ID]
	}
	return nil
}

// loadAttributes loads all attributes, expect for attachments and comments
func (conversations ConversationList) LoadAttributes(ctx context.Context) error {
	if _, err := conversations.LoadRepositories(ctx); err != nil {
		return fmt.Errorf("conversation.loadAttributes: LoadRepositories: %w", err)
	}
	return nil
}

// LoadComments loads comments
func (conversations ConversationList) LoadComments(ctx context.Context) error {
	return conversations.loadComments(ctx, builder.NewCond())
}

// LoadDiscussComments loads discuss comments
func (conversations ConversationList) LoadDiscussComments(ctx context.Context) error {
	return conversations.loadComments(ctx, builder.Eq{"comment.type": CommentTypeComment})
}

func getPostersByIDs(ctx context.Context, posterIDs []int64) (map[int64]*user_model.User, error) {
	posterMaps := make(map[int64]*user_model.User, len(posterIDs))
	left := len(posterIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", posterIDs[:limit]).
			Find(&posterMaps)
		if err != nil {
			return nil, err
		}
		left -= limit
		posterIDs = posterIDs[limit:]
	}
	return posterMaps, nil
}

func getPoster(posterID int64, posterMaps map[int64]*user_model.User) *user_model.User {
	if posterID == user_model.ActionsUserID {
		return user_model.NewActionsUser()
	}
	if posterID <= 0 {
		return nil
	}
	poster, ok := posterMaps[posterID]
	if !ok {
		return user_model.NewGhostUser()
	}
	return poster
}

func (conversations ConversationList) LoadIsRead(ctx context.Context, userID int64) error {
	conversationIDs := conversations.getConversationIDs()
	conversationUsers := make([]*ConversationUser, 0, len(conversationIDs))
	if err := db.GetEngine(ctx).Where("uid =?", userID).
		In("conversation_id").
		Find(&conversationUsers); err != nil {
		return err
	}

	for _, conversationUser := range conversationUsers {
		for _, conversation := range conversations {
			if conversation.ID == conversationUser.ConversationID {
				conversation.IsRead = conversationUser.IsRead
			}
		}
	}

	return nil
}
