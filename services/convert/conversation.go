// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	conversations_model "code.gitea.io/gitea/models/conversations"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

func ToConversation(ctx context.Context, conversation *conversations_model.Conversation) *api.Conversation {
	return toConversation(ctx, conversation)
}

// ToAPIConversation converts an Conversation to API format
func ToAPIConversation(ctx context.Context, conversation *conversations_model.Conversation) *api.Conversation {
	return toConversation(ctx, conversation)
}

func toConversation(ctx context.Context, conversation *conversations_model.Conversation) *api.Conversation {
	if err := conversation.LoadRepo(ctx); err != nil {
		return &api.Conversation{}
	}
	if err := conversation.LoadAttachments(ctx); err != nil {
		return &api.Conversation{}
	}

	apiConversation := &api.Conversation{
		ID:       conversation.ID,
		Index:    conversation.Index,
		IsLocked: conversation.IsLocked,
		Comments: conversation.NumComments,
		Created:  conversation.CreatedUnix.AsTime(),
		Updated:  conversation.UpdatedUnix.AsTime(),
	}

	if conversation.Repo != nil {
		if err := conversation.Repo.LoadOwner(ctx); err != nil {
			return &api.Conversation{}
		}
		apiConversation.URL = conversation.APIURL(ctx)
		apiConversation.HTMLURL = conversation.HTMLURL()

		apiConversation.Repo = &api.RepositoryMeta{
			ID:       conversation.Repo.ID,
			Name:     conversation.Repo.Name,
			Owner:    conversation.Repo.OwnerName,
			FullName: conversation.Repo.FullName(),
		}
	}

	if conversation.LockedUnix != 0 {
		apiConversation.Locked = conversation.LockedUnix.AsTimePtr()
	}

	return apiConversation
}

// ToConversationList converts an ConversationList to API format
func ToConversationList(ctx context.Context, doer *user_model.User, il conversations_model.ConversationList) []*api.Conversation {
	result := make([]*api.Conversation, len(il))
	for i := range il {
		result[i] = ToConversation(ctx, il[i])
	}
	return result
}

// ToAPIConversationList converts an ConversationList to API format
func ToAPIConversationList(ctx context.Context, doer *user_model.User, il conversations_model.ConversationList) []*api.Conversation {
	result := make([]*api.Conversation, len(il))
	for i := range il {
		result[i] = ToAPIConversation(ctx, il[i])
	}
	return result
}
