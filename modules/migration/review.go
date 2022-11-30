// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "time"

// Reviewable can be reviewed
type Reviewable interface {
	GetLocalIndex() int64
	GetForeignIndex() int64
}

// enumerate all review states
const (
	ReviewStatePending          = "PENDING"
	ReviewStateApproved         = "APPROVED"
	ReviewStateChangesRequested = "CHANGES_REQUESTED"
	ReviewStateCommented        = "COMMENTED"
	ReviewStateRequestReview    = "REQUEST_REVIEW"
)

// Review is a standard review information
type Review struct {
	ID           int64
	IssueIndex   int64  `yaml:"issue_index"`
	ReviewerID   int64  `yaml:"reviewer_id"`
	ReviewerName string `yaml:"reviewer_name"`
	Official     bool
	CommitID     string `yaml:"commit_id"`
	Content      string
	CreatedAt    time.Time `yaml:"created_at"`
	State        string    // PENDING, APPROVED, REQUEST_CHANGES, or COMMENT
	Comments     []*ReviewComment
}

// GetExternalName ExternalUserMigrated interface
func (r *Review) GetExternalName() string { return r.ReviewerName }

// ExternalID ExternalUserMigrated interface
func (r *Review) GetExternalID() int64 { return r.ReviewerID }

// ReviewComment represents a review comment
type ReviewComment struct {
	ID        int64
	InReplyTo int64 `yaml:"in_reply_to"`
	Content   string
	TreePath  string `yaml:"tree_path"`
	DiffHunk  string `yaml:"diff_hunk"`
	Position  int
	Line      int
	CommitID  string `yaml:"commit_id"`
	PosterID  int64  `yaml:"poster_id"`
	Reactions []*Reaction
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
}
