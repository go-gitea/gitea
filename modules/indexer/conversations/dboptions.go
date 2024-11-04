// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/modules/optional"
)

func ToSearchOptions(keyword string, opts *conversations_model.ConversationsOptions) *SearchOptions {
	searchOpt := &SearchOptions{
		Keyword:   keyword,
		RepoIDs:   opts.RepoIDs,
		AllPublic: opts.AllPublic,
	}

	if opts.UpdatedAfterUnix > 0 {
		searchOpt.UpdatedAfterUnix = optional.Some(opts.UpdatedAfterUnix)
	}
	if opts.UpdatedBeforeUnix > 0 {
		searchOpt.UpdatedBeforeUnix = optional.Some(opts.UpdatedBeforeUnix)
	}

	searchOpt.Paginator = opts.Paginator

	switch opts.SortType {
	case "", "latest":
		searchOpt.SortBy = SortByCreatedDesc
	case "oldest":
		searchOpt.SortBy = SortByCreatedAsc
	case "recentupdate":
		searchOpt.SortBy = SortByUpdatedDesc
	case "leastupdate":
		searchOpt.SortBy = SortByUpdatedAsc
	case "mostcomment":
		searchOpt.SortBy = SortByCommentsDesc
	case "leastcomment":
		searchOpt.SortBy = SortByCommentsAsc
	case "nearduedate":
		searchOpt.SortBy = SortByDeadlineAsc
	case "farduedate":
		searchOpt.SortBy = SortByDeadlineDesc
	case "priority", "priorityrepo", "project-column-sorting":
		// Unsupported sort type for search
		fallthrough
	default:
		searchOpt.SortBy = SortByUpdatedDesc
	}

	return searchOpt
}
