// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// ReviewStateType review state type
//
// swagger:enum ReviewStateType
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
)

// ReviewStateUnknown is an internal sentinel for unknown review state, not a valid API value.
const ReviewStateUnknown = ""

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
	// First line of a multi-line comment.
	StartLine uint64 `json:"start_line,omitempty"`
	// Side of the first line of a multi-line comment, LEFT or RIGHT.
	StartSide string `json:"start_side,omitempty"`

	HTMLURL     string `json:"html_url"`
	HTMLPullURL string `json:"pull_request_url"`
}

// CreatePullReviewOptions are options to create a pull request review
type CreatePullReviewOptions struct {
	Event    ReviewStateType           `json:"event"`
	Body     string                    `json:"body"`
	CommitID string                    `json:"commit_id"`
	Comments []CreatePullReviewComment `json:"comments"`
}

// CreatePullReviewComment represent a review comment for creation api
type CreatePullReviewComment struct {
	// the tree path
	// required: true
	Path string `json:"path"`
	Body string `json:"body"`
	// if comment to old file line or 0
	OldLineNum int64 `json:"old_position"`
	// if comment to new file line or 0
	NewLineNum int64 `json:"new_position"`
	// First line of a multi-line comment, or 0 for a single-line comment.
	StartLine int64 `json:"start_line,omitempty"`
	// Side of start_line, LEFT or RIGHT. Defaults to the side of the end line.
	StartSide string `json:"start_side,omitempty"`
}

// CreatePullReviewCommentReplyOptions are options to reply to a pull request review comment
type CreatePullReviewCommentReplyOptions struct {
	Body string `json:"body" binding:"Required"`
}

// SubmitPullReviewOptions are options to submit a pending pull request review
type SubmitPullReviewOptions struct {
	Event ReviewStateType `json:"event"`
	Body  string          `json:"body"`
}

// DismissPullReviewOptions are options to dismiss a pull request review
type DismissPullReviewOptions struct {
	Message string `json:"message"`
	Priors  bool   `json:"priors"`
}

// PullReviewRequestOptions are options to add or remove pull request review requests
type PullReviewRequestOptions struct {
	Reviewers     []string `json:"reviewers"`
	TeamReviewers []string `json:"team_reviewers"`
}

// ApplyPullReviewSuggestionsOptions identifies the review comments to apply in one commit.
type ApplyPullReviewSuggestionsOptions struct {
	// IDs of the review comments containing the suggestions.
	// required: true
	CommentIDs []int64 `json:"comment_ids" binding:"Required"`
	// Expected head commit SHA. Defaults to the current head of the pull request.
	HeadCommitID string `json:"head_commit_id"`
	// Commit message. Defaults to a message based on the selected suggestions.
	Message string `json:"message"`
}

// ApplyPullReviewSuggestionsResponse identifies the resulting commit.
type ApplyPullReviewSuggestionsResponse struct {
	SHA string `json:"sha"`
}
