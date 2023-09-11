// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64 `json:"id"`
	RepoID   int64 `json:"repo_id"`
	IsPublic bool  `json:"is_public"` // If the repo is public

	// Fields used for keyword searching
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Comments []string `json:"comments"`

	// Fields used for filtering
	IsPull             bool               `json:"is_pull"`
	IsClosed           bool               `json:"is_closed"`
	LabelIDs           []int64            `json:"label_ids"`
	NoLabel            bool               `json:"no_label"` // True if LabelIDs is empty
	MilestoneID        int64              `json:"milestone_id"`
	ProjectID          int64              `json:"project_id"`
	ProjectBoardID     int64              `json:"project_board_id"`
	PosterID           int64              `json:"poster_id"`
	AssigneeID         int64              `json:"assignee_id"`
	MentionIDs         []int64            `json:"mention_ids"`
	ReviewedIDs        []int64            `json:"reviewed_ids"`
	ReviewRequestedIDs []int64            `json:"review_requested_ids"`
	SubscriberIDs      []int64            `json:"subscriber_ids"`
	UpdatedUnix        timeutil.TimeStamp `json:"updated_unix"`

	// Fields used for sorting
	// UpdatedUnix is both used for filtering and sorting.
	// ID is used for sorting too, to make the sorting stable.
	CreatedUnix  timeutil.TimeStamp `json:"created_unix"`
	DeadlineUnix timeutil.TimeStamp `json:"deadline_unix"`
	CommentCount int64              `json:"comment_count"`
}

// Match represents on search result
type Match struct {
	ID    int64   `json:"id"`
	Score float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int64
	Hits  []Match
}

// SearchOptions represents search options.
//
// It has a slightly different design from database query options.
// In database query options, a field is never a pointer, so it could be confusing when it's zero value:
// Do you want to find data with a field value of 0, or do you not specify the field in the options?
// To avoid this confusion, db introduced db.NoConditionID(-1).
// So zero value means the field is not specified in the search options, and db.NoConditionID means "== 0" or "id NOT IN (SELECT id FROM ...)"
// It's still not ideal, it trapped developers many times.
// And sometimes -1 could be a valid value, like issue ID, negative numbers indicate exclusion.
// Since db.NoConditionID is for "db" (the package name is db), it makes sense not to use it in the indexer:
// Why do bleve/elasticsearch/meilisearch indexers need to know about db.NoConditionID?
// So in SearchOptions, we use pointer for fields which could be not specified,
// and always use the value to filter if it's not nil, even if it's zero or negative.
// It can handle almost all cases, if there is an exception, we can add a new field, like NoLabelOnly.
// Unfortunately, we still use db for the indexer and have to convert between db.NoConditionID and nil for legacy reasons.
type SearchOptions struct {
	Keyword string // keyword to search

	RepoIDs   []int64 // repository IDs which the issues belong to
	AllPublic bool    // if include all public repositories

	IsPull   util.OptionalBool // if the issues is a pull request
	IsClosed util.OptionalBool // if the issues is closed

	IncludedLabelIDs    []int64 // labels the issues have
	ExcludedLabelIDs    []int64 // labels the issues don't have
	IncludedAnyLabelIDs []int64 // labels the issues have at least one. It will be ignored if IncludedLabelIDs is not empty. It's an uncommon filter, but it has been supported accidentally by issues.IssuesOptions.IncludedLabelNames.
	NoLabelOnly         bool    // if the issues have no label, if true, IncludedLabelIDs and ExcludedLabelIDs, IncludedAnyLabelIDs will be ignored

	MilestoneIDs []int64 // milestones the issues have

	ProjectID      *int64 // project the issues belong to
	ProjectBoardID *int64 // project board the issues belong to

	PosterID *int64 // poster of the issues

	AssigneeID *int64 // assignee of the issues, zero means no assignee

	MentionID *int64 // mentioned user of the issues

	ReviewedID        *int64 // reviewer of the issues
	ReviewRequestedID *int64 // requested reviewer of the issues

	SubscriberID *int64 // subscriber of the issues

	UpdatedAfterUnix  *int64
	UpdatedBeforeUnix *int64

	db.Paginator

	SortBy SortBy // sort by field
}

// Copy returns a copy of the options.
// Be careful, it's not a deep copy, so `SearchOptions.RepoIDs = {...}` is OK while `SearchOptions.RepoIDs[0] = ...` is not.
func (o *SearchOptions) Copy(edit ...func(options *SearchOptions)) *SearchOptions {
	if o == nil {
		return nil
	}
	v := *o
	for _, e := range edit {
		e(&v)
	}
	return &v
}

type SortBy string

const (
	SortByCreatedDesc  SortBy = "-created_unix"
	SortByUpdatedDesc  SortBy = "-updated_unix"
	SortByCommentsDesc SortBy = "-comment_count"
	SortByDeadlineDesc SortBy = "-deadline_unix"
	SortByCreatedAsc   SortBy = "created_unix"
	SortByUpdatedAsc   SortBy = "updated_unix"
	SortByCommentsAsc  SortBy = "comment_count"
	SortByDeadlineAsc  SortBy = "deadline_unix"
	// Unsupported sort types which are supported by issues.IssuesOptions.SortType:
	//
	//  - "priorityrepo":
	//                    It's impossible to support it in the indexer.
	//                    It is based on the specified repository in the request, so we cannot add static field to the indexer.
	//                    If we do something like that query the issues in the specified repository first then append other issues,
	//                    it will break the pagination.
	//
	// - "project-column-sorting":
	//                    Although it's possible to support it by adding project.ProjectIssue.Sorting to the indexer,
	//                    but what if the issue belongs to multiple projects?
	//                    Since it's unsupported to search issues with keyword in project page, we don't need to support it.
)
