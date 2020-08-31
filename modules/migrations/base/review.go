// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// enumerate all review states
const (
	ReviewStatePending          = "PENDING"
	ReviewStateApproved         = "APPROVED"
	ReviewStateChangesRequested = "CHANGES_REQUESTED"
	ReviewStateCommented        = "COMMENTED"
)

// Review is a standard review information
type Review struct {
	ID           int64
	IssueIndex   int64
	ReviewerID   int64
	ReviewerName string
	Official     bool
	CommitID     string
	Content      string
	CreatedAt    time.Time
	State        string // PENDING, APPROVED, REQUEST_CHANGES, or COMMENT
	Comments     []*ReviewComment
}

// ReviewComment represents a review comment
type ReviewComment struct {
	ID        int64
	InReplyTo int64
	Content   string
	TreePath  string
	DiffHunk  string
	Position  int
	Line      int
	CommitID  string
	PosterID  int64
	Reactions []*Reaction
	CreatedAt time.Time
	UpdatedAt time.Time
}
