// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	ID               int64  `json:"id"`
	HTMLURL          string `json:"html_url"`
	PRURL            string `json:"pull_request_url"`
	IssueURL         string `json:"issue_url"`
	Poster           *User  `json:"user"`
	OriginalAuthor   string `json:"original_author"`
	OriginalAuthorID int64  `json:"original_author_id"`
	Body             string `json:"body"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	// required:true
	Body string `json:"body" binding:"Required"`
}

// EditIssueCommentOption options for editing a comment
type EditIssueCommentOption struct {
	// required: true
	Body string `json:"body" binding:"Required"`
}

// TimelineComment represents a timeline comment (comment of any type) on a commit or issue
type TimelineComment struct {
	ID int64 `json:"id"`

	// Type specifies the type of an event.
	// 0 Plain comment, can be associated with a commit (CommitID > 0) and a line (LineNum > 0)
	// 1 Reopen issue/pull request
	// 2 Close issue/pull request
	// 3 References.
	// 4 Reference from a commit (not part of a pull request)
	// 5 Reference from a comment
	// 6 Reference from a pull request
	// 7 Labels changed (if Body is "1", label was added, if not it was removed)
	// 8 Milestone changed
	// 9 Assignees changed
	// 10 Change Title
	// 11 Delete Branch
	// 12 Start a stopwatch for time tracking
	// 13 Stop a stopwatch for time tracking
	// 14 Add time manual for time tracking
	// 15 Cancel a stopwatch for time tracking
	// 16 Added a due date
	// 17 Modified the due date
	// 18 Removed a due date
	// 19 Dependency added
	// 20 Dependency removed
	// 21 Not returned; use review API to get more information
	// 22 Reviews a pull request by giving general feedback; use review API to get more information
	// 23 Lock an issue, giving only collaborators access
	// 24 Unlocks a previously locked issue
	// 25 Change pull request's target branch
	// 26 Delete time manual for time tracking
	// 27 add or remove Request from one
	// 28 merge pull request
	// 29 push to PR head branch (information about the push is included in Body)
	// 30 Project changed
	// 31 Project board changed
	// 32 Dismiss Review
	Type int64 `json:"type"`

	HTMLURL  string `json:"html_url"`
	PRURL    string `json:"pull_request_url"`
	IssueURL string `json:"issue_url"`
	Poster   *User  `json:"user"`
	Body     string `json:"body"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`

	OldProjectID int64        `json:"old_project_id"`
	ProjectID    int64        `json:"project_id"`
	OldMilestone *Milestone   `json:"old_milestone"`
	Milestone    *Milestone   `json:"milestone"`
	Time         *TrackedTime `json:"time"`
	OldTitle     string       `json:"old_title"`
	NewTitle     string       `json:"new_title"`
	OldRef       string       `json:"old_ref"`
	NewRef       string       `json:"new_ref"`

	RefIssue   *Issue   `json:"ref_issue"`
	RefComment *Comment `json:"ref_comment"`
	// action that was used to reference issue/PR
	// 0 means the cross-reference is simply a comment
	// 1 means the cross-reference should close an issue if it is resolved
	// 2 means the cross-reference should reopen an issue if it is resolved
	// 3 means the cross-reference will no longer affect the source
	RefAction int64 `json:"ref_action"`
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
