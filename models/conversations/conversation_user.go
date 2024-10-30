// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
)

// ConversationUser represents an conversation-user relation.
type ConversationUser struct {
	ID             int64 `xorm:"pk autoincr"`
	UID            int64 `xorm:"INDEX unique(uid_to_conversation)"` // User ID.
	ConversationID int64 `xorm:"INDEX unique(uid_to_conversation)"`
	IsRead         bool
	IsMentioned    bool
}

func init() {
	db.RegisterModel(new(ConversationUser))
}

// UpdateConversationUserByRead updates conversation-user relation for reading.
func UpdateConversationUserByRead(ctx context.Context, uid, conversationID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `conversation_user` SET is_read=? WHERE uid=? AND conversation_id=?", true, uid, conversationID)
	return err
}

// UpdateConversationUsersByMentions updates conversation-user pairs by mentioning.
func UpdateConversationUsersByMentions(ctx context.Context, conversationID int64, uids []int64) error {
	for _, uid := range uids {
		iu := &ConversationUser{
			UID:            uid,
			ConversationID: conversationID,
		}
		has, err := db.GetEngine(ctx).Get(iu)
		if err != nil {
			return err
		}

		iu.IsMentioned = true
		if has {
			_, err = db.GetEngine(ctx).ID(iu.ID).Cols("is_mentioned").Update(iu)
		} else {
			_, err = db.GetEngine(ctx).Insert(iu)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// GetConversationMentionIDs returns all mentioned user IDs of an conversation.
func GetConversationMentionIDs(ctx context.Context, conversationID int64) ([]int64, error) {
	var ids []int64
	return ids, db.GetEngine(ctx).Table(ConversationUser{}).
		Where("conversation_id=?", conversationID).
		And("is_mentioned=?", true).
		Select("uid").
		Find(&ids)
}

// NewConversationUsers inserts an conversation related users
func NewConversationUsers(ctx context.Context, repo *repo_model.Repository, conversation *Conversation) error {
	// Leave a seat for poster itself to append later, but if poster is one of assignee
	// and just waste 1 unit is cheaper than re-allocate memory once.
	conversationUsers := make([]*ConversationUser, 0, 1)

	conversationUsers = append(conversationUsers, &ConversationUser{
		ConversationID: conversation.ID,
	})

	return db.Insert(ctx, conversationUsers)
}
