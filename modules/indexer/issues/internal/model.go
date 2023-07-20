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

	// Imprecise indicates that the result is not accurate, and it needs second filtering and sorting by database.
	// It could be removed when all engines support filtering and sorting.
	Imprecise bool
}

// SearchOptions represents search options
type SearchOptions struct {
	Keyword string // keyword to search

	RepoIDs   []int64 // repository IDs which the issues belong to
	AllPublic bool    // if include all public repositories

	IsPull   util.OptionalBool // if the issues is a pull request
	IsClosed util.OptionalBool // if the issues is closed

	IncludedLabelIDs []int64 // labels the issues have
	ExcludedLabelIDs []int64 // labels the issues don't have
	NoLabelOnly      bool    // if the issues have no label, if true, IncludedLabelIDs and ExcludedLabelIDs will be ignored

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
	//    - "priorityrepo"
	//    - "project-column-sorting"
)
