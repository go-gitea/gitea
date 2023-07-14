// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/util"
)

// reFilter filters the given issuesIDs by the database.
// It is used to filter out issues coming from some indexers that are not supported fining filtering.
// Once all indexers support filtering, this function and this file can be removed.
func reFilter(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error) {
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
			return []int64{0} // It's zero, not db.NoConditionID,
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
	case SearchOptionsSortByCreatedAsc:
		sortType = "oldest"
	case SearchOptionsSortByUpdatedAsc:
		sortType = "leastupdate"
	case SearchOptionsSortByCommentsAsc:
		sortType = "leastcomment"
	case SearchOptionsSortByDueAsc:
		sortType = "farduedate"
	case SearchOptionsSortByCreatedDesc:
		sortType = "" // default
	case SearchOptionsSortByUpdatedDesc:
		sortType = "recentupdate"
	case SearchOptionsSortByCommentsDesc:
		sortType = "mostcomment"
	case SearchOptionsSortByDueDesc:
		sortType = "nearduedate"
	}

	opts := &issue_model.IssuesOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		RepoIDs:            nil, // it's unnecessary since issuesIDs are already filtered by repoIDs
		RepoCond:           nil, // it's unnecessary since issuesIDs are already filtered by repoIDs
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
		IncludedLabelNames: nil, // use LabelIDs instead
		ExcludedLabelNames: nil, // use LabelIDs instead
		IncludeMilestones:  nil, // use MilestoneIDs instead
		SortType:           sortType,
		IssueIDs:           issuesIDs,
		UpdatedAfterUnix:   convertInt64(options.UpdatedAfterUnix),
		UpdatedBeforeUnix:  convertInt64(options.UpdatedBeforeUnix),
		PriorityRepoID:     0,   // don't use priority repo since it isn't supported by search to sort by priorityrepo
		IsArchived:         0,   // it's unnecessary since issuesIDs are already filtered by repoIDs
		Org:                nil, // it's unnecessary since issuesIDs are already filtered by repoIDs
		Team:               nil, // it's unnecessary since issuesIDs are already filtered by repoIDs
		User:               nil, // it's unnecessary since issuesIDs are already filtered by repoIDs
	}
	// TODO: use a new function which returns ids only, to avoid unnecessary issues loading
	issues, err := issue_model.Issues(ctx, opts)
	if err != nil {
		return nil, err
	}
	issueIDs := make([]int64, 0, len(issues))
	for _, issue := range issues {
		issueIDs = append(issueIDs, issue.ID)
	}
	return issueIDs, nil
}
