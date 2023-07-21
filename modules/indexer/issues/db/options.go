// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

func ToDBOptions(options *internal.SearchOptions) *issue_model.IssuesOptions {
	convertID := func(id *int64) int64 {
		if id == nil {
			return 0
		}
		if *id == 0 {
			return db.NoConditionID
		}
		return *id
	}
	convertIDs := func(ids []int64) []int64 {
		if len(ids) == 1 && ids[0] == 0 {
			return []int64{db.NoConditionID}
		}
		return ids
	}
	convertLabelIDs := func(includes, excludes []int64, noLabelOnly bool) []int64 {
		if noLabelOnly {
			return []int64{0} // Be careful, it's zero, not db.NoConditionID
		}
		ret := make([]int64, 0, len(includes)+len(excludes))
		ret = append(ret, includes...)
		for _, id := range excludes {
			ret = append(ret, -id)
		}
		return ret
	}
	convertInt64 := func(i *int64) int64 {
		if i == nil {
			return 0
		}
		return *i
	}
	var sortType string
	switch options.SortBy {
	case internal.SortByCreatedAsc:
		sortType = "oldest"
	case internal.SortByUpdatedAsc:
		sortType = "leastupdate"
	case internal.SortByCommentsAsc:
		sortType = "leastcomment"
	case internal.SortByDeadlineAsc:
		sortType = "farduedate"
	case internal.SortByCreatedDesc:
		sortType = "newest"
	case internal.SortByUpdatedDesc:
		sortType = "recentupdate"
	case internal.SortByCommentsDesc:
		sortType = "mostcomment"
	case internal.SortByDeadlineDesc:
		sortType = "nearduedate"
	default:
		sortType = "newest"
	}

	opts := &issue_model.IssuesOptions{
		Paginator:          options.Paginator,
		RepoIDs:            options.RepoIDs,
		RepoCond:           nil,
		AssigneeID:         convertID(options.AssigneeID),
		PosterID:           convertID(options.PosterID),
		MentionedID:        convertID(options.MentionID),
		ReviewRequestedID:  convertID(options.ReviewRequestedID),
		ReviewedID:         convertID(options.ReviewedID),
		SubscriberID:       convertID(options.SubscriberID),
		MilestoneIDs:       convertIDs(options.MilestoneIDs),
		ProjectID:          convertID(options.ProjectID),
		ProjectBoardID:     convertID(options.ProjectBoardID),
		IsClosed:           options.IsClosed,
		IsPull:             options.IsPull,
		LabelIDs:           convertLabelIDs(options.IncludedLabelIDs, options.ExcludedLabelIDs, options.NoLabelOnly),
		IncludedLabelNames: nil,
		ExcludedLabelNames: nil,
		IncludeMilestones:  nil,
		SortType:           sortType,
		IssueIDs:           nil,
		UpdatedAfterUnix:   convertInt64(options.UpdatedAfterUnix),
		UpdatedBeforeUnix:  convertInt64(options.UpdatedBeforeUnix),
		PriorityRepoID:     0,
		IsArchived:         0,
		Org:                nil,
		Team:               nil,
		User:               nil,
	}
	return opts
}
