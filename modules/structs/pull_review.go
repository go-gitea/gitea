// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	// ReviewStateUnknown state of pr is unknown
	ReviewStateUnknown ReviewStateType = "UNKNOWN"
)

// PullReview represents a pull request review
type PullReview struct {
	ID       int64           `json:"id"`
	Reviewer *User           `json:"user"`
	State    ReviewStateType `json:"state"`
	Body     string          `json:"body"`
	CommitID string          `json:"commit_id"`
	Stale    bool            `json:"stale"`
	Official bool            `json:"official"`
	// swagger:strfmt date-time
	Submitted time.Time `json:"submitted_at"`

	// HTMLURL  string `json:"html_url"` // blocked by https://github.com/go-gitea/gitea/issues/11226
	HTMLPullURL string `json:"pull_request_url"`
}

// PullReviewComment represents a comment on a pull request
type PullReviewComment struct {
	ID       int64  `json:"id"`
	Body     string `json:"body"`
	Reviewer *User  `json:"user"`
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
