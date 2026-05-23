// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	// ID is the unique identifier for the comment
	ID int64 `json:"id"`
	// HTMLURL is the web URL for viewing the comment
	HTMLURL string `json:"html_url"`
	// PRURL is the API URL for the pull request (if applicable)
	PRURL string `json:"pull_request_url"`
	// IssueURL is the API URL for the issue
	IssueURL string `json:"issue_url"`
	// Poster is the user who posted the comment
	Poster *User `json:"user"`
	// OriginalAuthor is the original author name (for imported comments)
	OriginalAuthor string `json:"original_author"`
	// OriginalAuthorID is the original author ID (for imported comments)
	OriginalAuthorID int64 `json:"original_author_id"`
	// Body contains the comment text content
	Body string `json:"body"`
	// Attachments contains files attached to the comment
	Attachments []*Attachment `json:"assets"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	// required:true
	// Body is the comment text content
	Body string `json:"body" binding:"Required"`
}

// EditIssueCommentOption options for editing a comment
type EditIssueCommentOption struct {
	// required: true
	// Body is the updated comment text content
	Body string `json:"body" binding:"Required"`
}

// TimelineComment represents a timeline comment (comment of any type) on a commit or issue
type TimelineComment struct {
	// ID is the unique identifier for the timeline comment
	ID int64 `json:"id"`
	// Type indicates the type of timeline event
	Type string `json:"type"`

	// HTMLURL is the web URL for viewing the comment
	HTMLURL string `json:"html_url"`
	// PRURL is the API URL for the pull request (if applicable)
	PRURL string `json:"pull_request_url"`
	// IssueURL is the API URL for the issue
	IssueURL string `json:"issue_url"`
	// Poster is the user who created the timeline event
	Poster *User `json:"user"`
	// Body contains the timeline event content
	Body string `json:"body"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`

	OldProjectID int64        `json:"old_project_id"`
	ProjectID    int64        `json:"project_id"`
	OldMilestone *Milestone   `json:"old_milestone"`
	Milestone    *Milestone   `json:"milestone"`
	TrackedTime  *TrackedTime `json:"tracked_time"`
	OldTitle     string       `json:"old_title"`
	NewTitle     string       `json:"new_title"`
	OldRef       string       `json:"old_ref"`
	NewRef       string       `json:"new_ref"`

	RefIssue   *Issue   `json:"ref_issue"`
	RefComment *Comment `json:"ref_comment"`
	RefAction  string   `json:"ref_action"`
	// commit SHA where issue/PR was referenced
	RefCommitSHA string `json:"ref_commit_sha"`

	ReviewID int64 `json:"review_id"`

	Label *Label `json:"label"`

	Assignee     *User `json:"assignee"`
	AssigneeTeam *Team `json:"assignee_team"`
	// whether the assignees were removed or added
	RemovedAssignee bool `json:"removed_assignee"`

	ResolveDoer *User `json:"resolve_doer"`

	DependentIssue *Issue `json:"dependent_issue"`
}
