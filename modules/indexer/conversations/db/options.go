// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	conversation_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/optional"
)

func ToDBOptions(ctx context.Context, options *internal.SearchOptions) (*conversation_model.ConversationsOptions, error) {
	var sortType string
	switch options.SortBy {
	case internal.SortByCreatedAsc:
		sortType = "oldest"
	case internal.SortByUpdatedAsc:
		sortType = "leastupdate"
	case internal.SortByCommentsAsc:
		sortType = "leastcomment"
	case internal.SortByCreatedDesc:
		sortType = "newest"
	case internal.SortByUpdatedDesc:
		sortType = "recentupdate"
	case internal.SortByCommentsDesc:
		sortType = "mostcomment"
	default:
		sortType = "newest"
	}

	// See the comment of conversations_model.SearchOptions for the reason why we need to convert
	convertID := func(id optional.Option[int64]) int64 {
		if !id.Has() {
			return 0
		}
		value := id.Value()
		if value == 0 {
			return db.NoConditionID
		}
		return value
	}

	opts := &conversation_model.ConversationsOptions{
		Paginator:          options.Paginator,
		RepoIDs:            options.RepoIDs,
		AllPublic:          options.AllPublic,
		RepoCond:           nil,
		AssigneeID:         convertID(options.AssigneeID),
		PosterID:           convertID(options.PosterID),
		MentionedID:        convertID(options.MentionID),
		ReviewRequestedID:  convertID(options.ReviewRequestedID),
		ReviewedID:         convertID(options.ReviewedID),
		SubscriberID:       convertID(options.SubscriberID),
		ProjectID:          convertID(options.ProjectID),
		ProjectColumnID:    convertID(options.ProjectColumnID),
		IsClosed:           options.IsClosed,
		IsPull:             options.IsPull,
		IncludedLabelNames: nil,
		ExcludedLabelNames: nil,
		IncludeMilestones:  nil,
		SortType:           sortType,
		ConversationIDs:    nil,
		UpdatedAfterUnix:   options.UpdatedAfterUnix.Value(),
		UpdatedBeforeUnix:  options.UpdatedBeforeUnix.Value(),
		PriorityRepoID:     0,
		IsArchived:         optional.None[bool](),
		Org:                nil,
		Team:               nil,
		User:               nil,
	}

	return opts, nil
}
