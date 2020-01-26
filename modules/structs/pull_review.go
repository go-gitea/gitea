// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// PullRequestReview represents a pull request review
type PullRequestReview struct {
	ID       int64  `json:"id"`
	PRURL    string `json:"pull_request_url"`
	Reviewer *User  `json:"user"`
	Body     string `json:"body"`
	CommitID string `json:"commit_id"`
	Type     string `json:"type"`

	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`

	// TODO: is there a way to get a URL to the review itself?
	// HTMLURL  string `json:"html_url"`
}

// PullRequestReviewComment represents a comment on a pull request
type PullRequestReviewComment struct {
	ID         int64  `json:"id"`
	URL        string `json:"url"`
	PRURL      string `json:"pull_request_url"`
	ReviewID   int64  `json:"pull_request_review_id"`
	Path       string `json:"path"`
	CommitID   string `json:"commit_id"`
	DiffHunk   string `json:"diff_hunk"`
	LineNum    uint64 `json:"position"`
	OldLineNum uint64 `json:"original_position"`
	Reviewer   *User  `json:"user"`
	Body       string `json:"body"`
}
