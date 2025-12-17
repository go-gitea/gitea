// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// PullRequest represents a pull request
type PullRequest struct {
	// The unique identifier of the pull request
	ID int64 `json:"id"`
	// The API URL of the pull request
	URL string `json:"url"`
	// The pull request number
	Index int64 `json:"number"`
	// The user who created the pull request
	Poster *User `json:"user"`
	// The title of the pull request
	Title string `json:"title"`
	// The description body of the pull request
	Body string `json:"body"`
	// The labels attached to the pull request
	Labels []*Label `json:"labels"`
	// The milestone associated with the pull request
	Milestone *Milestone `json:"milestone"`
	// The primary assignee of the pull request
	Assignee *User `json:"assignee"`
	// The list of users assigned to the pull request
	Assignees []*User `json:"assignees"`
	// The users requested to review the pull request
	RequestedReviewers []*User `json:"requested_reviewers"`
	// The teams requested to review the pull request
	RequestedReviewersTeams []*Team `json:"requested_reviewers_teams"`
	// The current state of the pull request
	State StateType `json:"state"`
	// Whether the pull request is a draft
	Draft bool `json:"draft"`
	// Whether the pull request conversation is locked
	IsLocked bool `json:"is_locked"`
	// The number of comments on the pull request
	Comments int `json:"comments"`

	// number of review comments made on the diff of a PR review (not including comments on commits or issues in a PR)
	ReviewComments int `json:"review_comments,omitempty"`

	// The number of lines added in the pull request
	Additions *int `json:"additions,omitempty"`
	// The number of lines deleted in the pull request
	Deletions *int `json:"deletions,omitempty"`
	// The number of files changed in the pull request
	ChangedFiles *int `json:"changed_files,omitempty"`

	// The HTML URL to view the pull request
	HTMLURL string `json:"html_url"`
	// The URL to download the diff patch
	DiffURL string `json:"diff_url"`
	// The URL to download the patch file
	PatchURL string `json:"patch_url"`

	// Whether the pull request can be merged
	Mergeable bool `json:"mergeable"`
	// Whether the pull request has been merged
	HasMerged bool `json:"merged"`
	// swagger:strfmt date-time
	Merged *time.Time `json:"merged_at"`
	// The SHA of the merge commit
	MergedCommitID *string `json:"merge_commit_sha"`
	// The user who merged the pull request
	MergedBy *User `json:"merged_by"`
	// Whether maintainers can edit the pull request
	AllowMaintainerEdit bool `json:"allow_maintainer_edit"`

	// Information about the base branch
	Base *PRBranchInfo `json:"base"`
	// Information about the head branch
	Head *PRBranchInfo `json:"head"`
	// The merge base commit SHA
	MergeBase string `json:"merge_base"`

	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

	// swagger:strfmt date-time
	Created *time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`

	// The pin order for the pull request
	PinOrder int `json:"pin_order"`
}

// PRBranchInfo information about a branch
type PRBranchInfo struct {
	// The display name of the branch
	Name string `json:"label"`
	// The git reference of the branch
	Ref string `json:"ref"`
	// The commit SHA of the branch head
	Sha string `json:"sha"`
	// The unique identifier of the repository
	RepoID int64 `json:"repo_id"`
	// The repository information
	Repository *Repository `json:"repo"`
}

// ListPullRequestsOptions options for listing pull requests
type ListPullRequestsOptions struct {
	// The page number for pagination
	Page int `json:"page"`
	// The state filter for pull requests
	State string `json:"state"`
}

// CreatePullRequestOption options when creating a pull request
type CreatePullRequestOption struct {
	// The head branch for the pull request, it could be a branch name on the base repository or
	// a form like `<username>:<branch>` which refers to the user's fork repository's branch.
	Head string `json:"head" binding:"Required"`
	// The base branch for the pull request
	Base string `json:"base" binding:"Required"`
	// The title of the pull request
	Title string `json:"title" binding:"Required"`
	// The description body of the pull request
	Body string `json:"body"`
	// The primary assignee username
	Assignee string `json:"assignee"`
	// The list of assignee usernames
	Assignees []string `json:"assignees"`
	// The milestone ID to assign to the pull request
	Milestone int64 `json:"milestone"`
	// The list of label IDs to assign to the pull request
	Labels []int64 `json:"labels"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
	// The list of reviewer usernames
	Reviewers []string `json:"reviewers"`
	// The list of team reviewer names
	TeamReviewers []string `json:"team_reviewers"`
}

// EditPullRequestOption options when modify pull request
type EditPullRequestOption struct {
	// The new title for the pull request
	Title string `json:"title"`
	// The new description body for the pull request
	Body *string `json:"body"`
	// The new base branch for the pull request
	Base string `json:"base"`
	// The new primary assignee username
	Assignee string `json:"assignee"`
	// The new list of assignee usernames
	Assignees []string `json:"assignees"`
	// The new milestone ID for the pull request
	Milestone int64 `json:"milestone"`
	// The new list of label IDs for the pull request
	Labels []int64 `json:"labels"`
	// The new state for the pull request
	State *string `json:"state"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
	// Whether to remove the current deadline
	RemoveDeadline *bool `json:"unset_due_date"`
	// Whether to allow maintainer edits
	AllowMaintainerEdit *bool `json:"allow_maintainer_edit"`
}

// ChangedFile store information about files affected by the pull request
type ChangedFile struct {
	// The name of the changed file
	Filename string `json:"filename"`
	// The previous filename if the file was renamed
	PreviousFilename string `json:"previous_filename,omitempty"`
	// The status of the file change (added, modified, deleted, etc.)
	Status string `json:"status"`
	// The number of lines added to the file
	Additions int `json:"additions"`
	// The number of lines deleted from the file
	Deletions int `json:"deletions"`
	// The total number of changes to the file
	Changes int `json:"changes"`
	// The HTML URL to view the file changes
	HTMLURL string `json:"html_url,omitempty"`
	// The API URL to get the file contents
	ContentsURL string `json:"contents_url,omitempty"`
	// The raw URL to download the file
	RawURL string `json:"raw_url,omitempty"`
}
