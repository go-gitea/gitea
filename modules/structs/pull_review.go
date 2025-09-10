// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// ReviewStateType review state type
type ReviewStateType string

const (
	// ReviewStateApproved pr is approved
	ReviewStateApproved ReviewStateType = "APPROVED"
	// ReviewStatePending pr state is pending
	ReviewStatePending ReviewStateType = "PENDING"
	// ReviewStateComment is a comment review
	ReviewStateComment ReviewStateType = "COMMENT"
	// ReviewStateRequestChanges changes for pr are requested
	ReviewStateRequestChanges ReviewStateType = "REQUEST_CHANGES"
	// ReviewStateRequestReview review is requested from user
	ReviewStateRequestReview ReviewStateType = "REQUEST_REVIEW"
	// ReviewStateUnknown state of pr is unknown
	ReviewStateUnknown ReviewStateType = ""
)

// PullReview represents a pull request review
type PullReview struct {
	ID                int64           `json:"id"`
	Reviewer          *User           `json:"user"`
	ReviewerTeam      *Team           `json:"team"`
	State             ReviewStateType `json:"state"`
	Body              string          `json:"body"`
	CommitID          string          `json:"commit_id"`
	Stale             bool            `json:"stale"`
	Official          bool            `json:"official"`
	Dismissed         bool            `json:"dismissed"`
	CodeCommentsCount int             `json:"comments_count"`
	// swagger:strfmt date-time
	Submitted time.Time `json:"submitted_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`

	// HTMLURL is the web URL for viewing the review
	HTMLURL string `json:"html_url"`
	// HTMLPullURL is the web URL for the pull request
	HTMLPullURL string `json:"pull_request_url"`
}

// PullReviewComment represents a comment on a pull request review
type PullReviewComment struct {
	ID       int64  `json:"id"`
	Body     string `json:"body"`
	Poster   *User  `json:"user"`
	Resolver *User  `json:"resolver"`
	ReviewID int64  `json:"pull_request_review_id"`

	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`

	Path         string `json:"path"`
	CommitID     string `json:"commit_id"`
	OrigCommitID string `json:"original_commit_id"`
	DiffHunk     string `json:"diff_hunk"`
	LineNum      uint64 `json:"position"`
	OldLineNum   uint64 `json:"original_position"`

	HTMLURL     string `json:"html_url"`
	HTMLPullURL string `json:"pull_request_url"`
}

// CreatePullReviewOptions are options to create a pull review
type CreatePullReviewOptions struct {
	Event    ReviewStateType           `json:"event"`
	Body     string                    `json:"body"`
	CommitID string                    `json:"commit_id"`
	Comments []CreatePullReviewComment `json:"comments"`
}

// CreatePullReviewComment represent a review comment for creation api
type CreatePullReviewComment struct {
	// the tree path
	Path string `json:"path"`
	Body string `json:"body"`
	// if comment to old file line or 0
	OldLineNum int64 `json:"old_position"`
	// if comment to new file line or 0
	NewLineNum int64 `json:"new_position"`
}

// SubmitPullReviewOptions are options to submit a pending pull review
type SubmitPullReviewOptions struct {
	Event ReviewStateType `json:"event"`
	Body  string          `json:"body"`
}

// DismissPullReviewOptions are options to dismiss a pull review
type DismissPullReviewOptions struct {
	Message string `json:"message"`
	Priors  bool   `json:"priors"`
}

// PullReviewRequestOptions are options to add or remove pull review requests
type PullReviewRequestOptions struct {
	Reviewers     []string `json:"reviewers"`
	TeamReviewers []string `json:"team_reviewers"`
}
