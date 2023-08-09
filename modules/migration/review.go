// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "time"

// Reviewable can be reviewed
type Reviewable interface {
	GetLocalIndex() int64

	// GetForeignIndex presents the foreign index, which could be misused:
	// For example, if there are 2 Gitea sites: site-A exports a dataset, then site-B imports it:
	// * if site-A exports files by using its LocalIndex
	// * from site-A's view, LocalIndex is site-A's IssueIndex while ForeignIndex is site-B's IssueIndex
	// * but from site-B's view, LocalIndex is site-B's IssueIndex while ForeignIndex is site-A's IssueIndex
	//
	// So the exporting/importing must be paired, but the meaning of them looks confusing then:
	// * either site-A and site-B both use LocalIndex during dumping/restoring
	// * or site-A and site-B both use ForeignIndex
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

// GetExternalID ExternalUserMigrated interface
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
