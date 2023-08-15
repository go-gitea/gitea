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

	if opts.ProjectID != 0 {
		searchOpt.ProjectID = &opts.ProjectID
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

	if opts.AssigneeID > 0 {
		searchOpt.AssigneeID = &opts.AssigneeID
	}
	if opts.PosterID > 0 {
		searchOpt.PosterID = &opts.PosterID
	}
	if opts.MentionedID > 0 {
		searchOpt.MentionID = &opts.MentionedID
	}
	if opts.ReviewedID > 0 {
		searchOpt.ReviewedID = &opts.ReviewedID
	}
	if opts.ReviewRequestedID > 0 {
		searchOpt.ReviewRequestedID = &opts.ReviewRequestedID
	}
	if opts.SubscriberID > 0 {
		searchOpt.SubscriberID = &opts.SubscriberID
	}

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
