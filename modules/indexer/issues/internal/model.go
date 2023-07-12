// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID     int64 `json:"id"`
	RepoID int64 `json:"repo_id"`

	// Fields used for keyword searching
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Comments []string `json:"comments"`

	// Fields used for filtering
	IsPull             bool     `json:"is_pull"`
	IsClosed           bool     `json:"is_closed"`     // So if the status of an issue has changed, we should reindex the issue.
	Labels             []string `json:"labels"`        // So if the labels of an issue have changed, we should reindex the issue.
	NoLabels           bool     `json:"no_labels"`     // True if Labels is empty
	Milestones         []int64  `json:"milestones"`    // So if the milestones of an issue have changed, we should reindex the issue.
	NoMilestones       bool     `json:"no_milestones"` // True if Milestones is empty
	Projects           []int64  `json:"projects"`      // So if the projects of an issue have changed, we should reindex the issue.
	NoProjects         bool     `json:"no_projects"`   // True if Projects is empty
	Author             int64    `json:"author"`        // So if the author of an issue has changed, we should reindex the issue.
	Assignee           int64    `json:"assignee"`      // So if the assignee of an issue has changed, we should reindex the issue.
	Mentions           []int64  `json:"mentions"`
	Reviewers          []int64  `json:"reviewers"`           // So if the reviewers of an issue have changed, we should reindex the issue.
	RequestedReviewers []int64  `json:"requested_reviewers"` // So if the requested reviewers of an issue have changed, we should reindex the issue.

	// Fields used for sorting
	CreatedAt    timeutil.TimeStamp `json:"created_at"`
	UpdatedAt    timeutil.TimeStamp `json:"updated_at"`
	CommentCount int64              `json:"comment_count"`
	DueDate      timeutil.TimeStamp `json:"due_date"`
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
// So the search engine should support:
//   - Filter by boolean/int value
//   - Filter by "array contains any of specified elements"
//   - Filter by "array doesn't contain any of specified elements"
type SearchOptions struct {
	Keyword string // keyword to search

	Repos []int64 // repository IDs which the issues belong to

	IsPull util.OptionalBool // if the issues is a pull request
	Closed util.OptionalBool // if the issues is closed

	Labels         []string // labels the issues have
	ExcludedLabels []string // labels the issues don't have
	NoLabels       bool     // if the issues have no labels

	Milestones   []int64 // milestones the issues have
	NoMilestones bool    // if the issues have no milestones

	Projects   []int64 // projects the issues belong to
	NoProjects bool    // if the issues have no projects

	Authors []int64 // authors of the issues

	Assignees   []int64 // assignees of the issues
	NoAssignees bool    // if the issues have no assignees

	Mentions []int64 // users mentioned in the issues

	Reviewers []int64 // reviewers of the issues

	RequestReviewers []int64 // users requested to review the issues

	Skip  int // skip the first N results
	Limit int // limit the number of results
}
