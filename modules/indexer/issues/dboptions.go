// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
)

func ToSearchOptions(keyword string, opts *issues_model.IssuesOptions) *SearchOptions {
	searchOpt := &SearchOptions{
		Keyword:   keyword,
		RepoIDs:   opts.RepoIDs,
		AllPublic: false,
		IsPull:    opts.IsPull,
		IsClosed:  opts.IsClosed,
	}

	if len(opts.LabelIDs) == 1 && opts.LabelIDs[0] == 0 {
		searchOpt.NoLabelOnly = true
	} else {
		for _, labelID := range opts.LabelIDs {
			if labelID > 0 {
				searchOpt.IncludedLabelIDs = append(searchOpt.IncludedLabelIDs, labelID)
			} else {
				searchOpt.ExcludedLabelIDs = append(searchOpt.ExcludedLabelIDs, -labelID)
			}
		}
		// opts.IncludedLabelNames and opts.ExcludedLabelNames are not supported here.
		// It's not a TO DO, it's just unnecessary.
	}

	if len(opts.MilestoneIDs) == 1 && opts.MilestoneIDs[0] == db.NoConditionID {
		searchOpt.MilestoneIDs = []int64{0}
	} else {
		searchOpt.MilestoneIDs = opts.MilestoneIDs
	}

	// See the comment of issues_model.SearchOptions for the reason why we need to convert
	convertID := func(id int64) *int64 {
		if id > 0 {
			return &id
		}
		if id == db.NoConditionID {
			var zero int64
			return &zero
		}
		return nil
	}

	searchOpt.ProjectID = convertID(opts.ProjectID)
	searchOpt.ProjectBoardID = convertID(opts.ProjectBoardID)
	searchOpt.PosterID = convertID(opts.PosterID)
	searchOpt.AssigneeID = convertID(opts.AssigneeID)
	searchOpt.MentionID = convertID(opts.MentionedID)
	searchOpt.ReviewedID = convertID(opts.ReviewedID)
	searchOpt.ReviewRequestedID = convertID(opts.ReviewRequestedID)
	searchOpt.SubscriberID = convertID(opts.SubscriberID)

	if opts.UpdatedAfterUnix > 0 {
		searchOpt.UpdatedAfterUnix = &opts.UpdatedAfterUnix
	}
	if opts.UpdatedBeforeUnix > 0 {
		searchOpt.UpdatedBeforeUnix = &opts.UpdatedBeforeUnix
	}

	searchOpt.Paginator = opts.Paginator

	switch opts.SortType {
	case "":
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
		searchOpt.SortBy = SortByUpdatedDesc
	default:
		searchOpt.SortBy = SortByUpdatedDesc
	}

	return searchOpt
}
