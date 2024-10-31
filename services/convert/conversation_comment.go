// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	conversations_model "code.gitea.io/gitea/models/conversations"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIComment converts a conversations_model.Comment to the api.Comment format for API usage
func ConversationToAPIComment(ctx context.Context, repo *repo_model.Repository, c *conversations_model.ConversationComment) *api.Comment {
	return &api.Comment{
		ID:              c.ID,
		Poster:          ToUser(ctx, c.Poster, nil),
		HTMLURL:         c.HTMLURL(ctx),
		ConversationURL: c.ConversationURL(ctx),
		Body:            c.Content,
		Attachments:     ToAPIAttachments(repo, c.Attachments),
		Created:         c.CreatedUnix.AsTime(),
		Updated:         c.UpdatedUnix.AsTime(),
	}
}

// ToTimelineComment converts a conversations_model.Comment to the api.TimelineComment format
func ConversationCommentToTimelineComment(ctx context.Context, repo *repo_model.Repository, c *conversations_model.ConversationComment, doer *user_model.User) *api.TimelineComment {
	comment := &api.TimelineComment{
		ID:              c.ID,
		Type:            c.Type.String(),
		Poster:          ToUser(ctx, c.Poster, nil),
		HTMLURL:         c.HTMLURL(ctx),
		ConversationURL: c.ConversationURL(ctx),
		Body:            c.Content,
		Created:         c.CreatedUnix.AsTime(),
		Updated:         c.UpdatedUnix.AsTime(),

		RefCommitSHA: c.Conversation.CommitSha,
	}

	return comment
}
