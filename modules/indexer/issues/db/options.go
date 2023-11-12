// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

func ToDBOptions(ctx context.Context, options *internal.SearchOptions) (*issue_model.IssuesOptions, error) {
	// See the comment of issues_model.SearchOptions for the reason why we need to convert
	convertID := func(id *int64) int64 {
		if id == nil {
			return 0
		}
		if *id == 0 {
			return db.NoConditionID
		}
		return *id
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
		ProjectID:          convertID(options.ProjectID),
		ProjectBoardID:     convertID(options.ProjectBoardID),
		IsClosed:           options.IsClosed,
		IsPull:             options.IsPull,
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

	if len(options.MilestoneIDs) == 1 && options.MilestoneIDs[0] == 0 {
		opts.MilestoneIDs = []int64{db.NoConditionID}
	} else {
		opts.MilestoneIDs = options.MilestoneIDs
	}

	if options.NoLabelOnly {
		opts.LabelIDs = []int64{0} // Be careful, it's zero, not db.NoConditionID
	} else {
		opts.LabelIDs = make([]int64, 0, len(options.IncludedLabelIDs)+len(options.ExcludedLabelIDs))
		opts.LabelIDs = append(opts.LabelIDs, options.IncludedLabelIDs...)
		for _, id := range options.ExcludedLabelIDs {
			opts.LabelIDs = append(opts.LabelIDs, -id)
		}

		if len(options.IncludedLabelIDs) == 0 && len(options.IncludedAnyLabelIDs) > 0 {
			labels, err := issue_model.GetLabelsByIDs(ctx, options.IncludedAnyLabelIDs, "name")
			if err != nil {
				return nil, fmt.Errorf("GetLabelsByIDs: %v", err)
			}
			set := container.Set[string]{}
			for _, label := range labels {
				if !set.Contains(label.Name) {
					set.Add(label.Name)
					opts.IncludedLabelNames = append(opts.IncludedLabelNames, label.Name)
				}
			}
		}
	}

	return opts, nil
}
