// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/util"
)

func ToDBOptions(options *internal.SearchOptions) *issue_model.IssuesOptions {
	convertID := func(id *int64) int64 {
		if id == nil {
			return db.NoConditionID
		}
		return *id
	}
	convertIDs := func(ids []int64, no bool) []int64 {
		if no {
			return []int64{db.NoConditionID}
		}
		return ids
	}
	convertBool := func(b *bool) util.OptionalBool {
		if b == nil {
			return util.OptionalBoolNone
		}
		return util.OptionalBoolOf(*b)
	}
	convertLabelIDs := func(includes, excludes []int64, no bool) []int64 {
		if no {
			return []int64{0} // Be careful, it's zero, not db.NoConditionID,
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
	sortType := ""
	switch options.SortBy {
	case internal.SearchOptionsSortByCreatedAsc:
		sortType = "oldest"
	case internal.SearchOptionsSortByUpdatedAsc:
		sortType = "leastupdate"
	case internal.SearchOptionsSortByCommentsAsc:
		sortType = "leastcomment"
	case internal.SearchOptionsSortByDueAsc:
		sortType = "farduedate"
	case internal.SearchOptionsSortByCreatedDesc:
		sortType = "" // default
	case internal.SearchOptionsSortByUpdatedDesc:
		sortType = "recentupdate"
	case internal.SearchOptionsSortByCommentsDesc:
		sortType = "mostcomment"
	case internal.SearchOptionsSortByDueDesc:
		sortType = "nearduedate"
	}

	opts := &issue_model.IssuesOptions{
		Paginator:          db.NewAbsoluteListOptions(options.Skip, options.Limit),
		RepoIDs:            options.RepoIDs,
		RepoCond:           nil,
		AssigneeID:         convertID(options.AssigneeID),
		PosterID:           convertID(options.PosterID),
		MentionedID:        convertID(options.MentionID),
		ReviewRequestedID:  convertID(options.ReviewRequestedID),
		ReviewedID:         convertID(options.ReviewedID),
		SubscriberID:       convertID(options.SubscriberID),
		MilestoneIDs:       convertIDs(options.MilestoneIDs, options.NoMilestone),
		ProjectID:          convertID(options.ProjectID),
		ProjectBoardID:     convertID(options.ProjectBoardID),
		IsClosed:           convertBool(options.IsClosed),
		IsPull:             convertBool(options.IsPull),
		LabelIDs:           convertLabelIDs(options.IncludedLabelIDs, options.ExcludedLabelIDs, options.NoLabel),
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
