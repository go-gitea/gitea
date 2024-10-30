// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversation

import (
	"context"

	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/storage"
)

// NewConversation creates new conversation with labels for repository.
func NewConversation(ctx context.Context, repo *repo_model.Repository, uuids []string, conversation *conversations_model.Conversation) error {
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		return conversations_model.NewConversation(ctx, repo, conversation, uuids)
	}); err != nil {
		return err
	}

	// notify_service.NewConversation(ctx, conversation, mentions)

	return nil
}

// DeleteConversation deletes an conversation
func DeleteConversation(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, conversation *conversations_model.Conversation) error {
	// load conversation before deleting it
	if err := conversation.LoadAttributes(ctx); err != nil {
		return err
	}

	// delete entries in database
	if err := deleteConversation(ctx, conversation); err != nil {
		return err
	}

	// notify_service.DeleteConversation(ctx, doer, conversation)

	return nil
}

// deleteConversation deletes the conversation
func deleteConversation(ctx context.Context, conversation *conversations_model.Conversation) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)
	if _, err := e.ID(conversation.ID).NoAutoCondition().Delete(conversation); err != nil {
		return err
	}

	// update the total conversation numbers
	if err := repo_model.UpdateRepoConversationNumbers(ctx, conversation.RepoID, false); err != nil {
		return err
	}
	// if the conversation is closed, update the closed conversation numbers
	if conversation.IsLocked {
		if err := repo_model.UpdateRepoConversationNumbers(ctx, conversation.RepoID, true); err != nil {
			return err
		}
	}

	// find attachments related to this conversation and remove them
	if err := conversation.LoadAttributes(ctx); err != nil {
		return err
	}

	for i := range conversation.Attachments {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete conversation attachment", conversation.Attachments[i].RelativePath())
	}

	// delete all database data still assigned to this conversation
	if err := db.DeleteBeans(ctx,
		&conversations_model.ConversationContentHistory{ConversationID: conversation.ID},
		&conversations_model.Comment{ConversationID: conversation.ID},
		&conversations_model.ConversationDependency{ConversationID: conversation.ID},
		&conversations_model.ConversationUser{ConversationID: conversation.ID},
		//&activities_model.Notification{ConversationID: conversation.ID},
		&conversations_model.CommentReaction{ConversationID: conversation.ID},
		&repo_model.Attachment{ConversationID: conversation.ID},
		&conversations_model.Comment{ConversationID: conversation.ID},
		&conversations_model.ConversationDependency{DependencyID: conversation.ID},
		&conversations_model.Comment{DependentConversationID: conversation.ID},
	); err != nil {
		return err
	}

	return committer.Commit()
}
