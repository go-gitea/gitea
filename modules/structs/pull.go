// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// PullRequest represents a pull request
type PullRequest struct {
	ID        int64      `json:"id"`
	URL       string     `json:"url"`
	Index     int64      `json:"number"`
	Poster    *User      `json:"user"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Labels    []*Label   `json:"labels"`
	Milestone *Milestone `json:"milestone"`
	Assignee  *User      `json:"assignee"`
	Assignees []*User    `json:"assignees"`
	State     StateType  `json:"state"`
	Comments  int        `json:"comments"`

	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`

	Mergeable bool `json:"mergeable"`
	HasMerged bool `json:"merged"`
	// swagger:strfmt date-time
	Merged         *time.Time `json:"merged_at"`
	MergedCommitID *string    `json:"merge_commit_sha"`
	MergedBy       *User      `json:"merged_by"`

	Base      *PRBranchInfo `json:"base"`
	Head      *PRBranchInfo `json:"head"`
	MergeBase string        `json:"merge_base"`

	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

	// swagger:strfmt date-time
	Created *time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
}

// PRBranchInfo information about a branch
type PRBranchInfo struct {
	Name       string      `json:"label"`
	Ref        string      `json:"ref"`
	Sha        string      `json:"sha"`
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `json:"repo"`
}

// ListPullRequestsOptions options for listing pull requests
type ListPullRequestsOptions struct {
	Page  int    `json:"page"`
	State string `json:"state"`
}

// CreatePullRequestOption options when creating a pull request
type CreatePullRequestOption struct {
	Head      string   `json:"head" binding:"Required"`
	Base      string   `json:"base" binding:"Required"`
	Title     string   `json:"title" binding:"Required"`
	Body      string   `json:"body"`
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone int64    `json:"milestone"`
	Labels    []int64  `json:"labels"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// EditPullRequestOption options when modify pull request
type EditPullRequestOption struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone int64    `json:"milestone"`
	Labels    []int64  `json:"labels"`
	State     *string  `json:"state"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}
