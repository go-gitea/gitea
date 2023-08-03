// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// PullRequest represents a pull request
type PullRequest struct {
	ID                 int64      `json:"id"`
	URL                string     `json:"url"`
	Index              int64      `json:"number"`
	Poster             *User      `json:"user"`
	Title              string     `json:"title"`
	Body               string     `json:"body"`
	Labels             []*Label   `json:"labels"`
	Milestone          *Milestone `json:"milestone"`
	Assignee           *User      `json:"assignee"`
	Assignees          []*User    `json:"assignees"`
	RequestedReviewers []*User    `json:"requested_reviewers"`
	State              StateType  `json:"state"`
	IsLocked           bool       `json:"is_locked"`
	Comments           int        `json:"comments"`

	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`

	Mergeable bool `json:"mergeable"`
	HasMerged bool `json:"merged"`
	// swagger:strfmt date-time
	Merged              *time.Time `json:"merged_at"`
	MergedCommitID      *string    `json:"merge_commit_sha"`
	MergedBy            *User      `json:"merged_by"`
	AllowMaintainerEdit bool       `json:"allow_maintainer_edit"`

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

	PinOrder int `json:"pin_order"`
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
	Base      string   `json:"base"`
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone int64    `json:"milestone"`
	Labels    []int64  `json:"labels"`
	State     *string  `json:"state"`
	// swagger:strfmt date-time
	Deadline            *time.Time `json:"due_date"`
	RemoveDeadline      *bool      `json:"unset_due_date"`
	AllowMaintainerEdit *bool      `json:"allow_maintainer_edit"`
}

// ChangedFile store information about files affected by the pull request
type ChangedFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename,omitempty"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	HTMLURL          string `json:"html_url,omitempty"`
	ContentsURL      string `json:"contents_url,omitempty"`
	RawURL           string `json:"raw_url,omitempty"`
}
