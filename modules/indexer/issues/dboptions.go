// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/optional"
)

func ToSearchOptions(keyword string, opts *issues_model.IssuesOptions) *SearchOptions {
	searchOpt := &SearchOptions{
		Keyword:    keyword,
		RepoIDs:    opts.RepoIDs,
		AllPublic:  opts.AllPublic,
		IsPull:     opts.IsPull,
		IsClosed:   opts.IsClosed,
		IsArchived: opts.IsArchived,
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

	if opts.ProjectID > 0 {
		searchOpt.ProjectID = optional.Some(opts.ProjectID)
	} else if opts.ProjectID == db.NoConditionID { // FIXME: this is inconsistent from other places
		searchOpt.ProjectID = optional.Some[int64](0) // Those issues with no project(projectid==0)
	}

	if opts.AssigneeID.Value() == db.NoConditionID {
		searchOpt.AssigneeID = optional.Some[int64](0) // FIXME: this is inconsistent from other places, 0 means "no assignee"
	} else if opts.AssigneeID.Value() != 0 {
		searchOpt.AssigneeID = opts.AssigneeID
	}

	// See the comment of issues_model.SearchOptions for the reason why we need to convert
	convertID := func(id int64) optional.Option[int64] {
		if id > 0 {
			return optional.Some(id)
		}
		if id == db.NoConditionID {
			return optional.None[int64]()
		}
		return nil
	}

	searchOpt.ProjectColumnID = convertID(opts.ProjectColumnID)
	searchOpt.PosterID = opts.PosterID
	searchOpt.MentionID = convertID(opts.MentionedID)
	searchOpt.ReviewedID = convertID(opts.ReviewedID)
	searchOpt.ReviewRequestedID = convertID(opts.ReviewRequestedID)
	searchOpt.SubscriberID = convertID(opts.SubscriberID)

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
