// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

type Activity struct {
	// The unique identifier of the activity
	ID int64 `json:"id"`
	// The ID of the user who receives/sees this activity
	UserID int64 `json:"user_id"` // Receiver user
	// the type of action
	//
	// enum: create_repo,rename_repo,star_repo,watch_repo,commit_repo,create_issue,create_pull_request,transfer_repo,push_tag,comment_issue,merge_pull_request,close_issue,reopen_issue,close_pull_request,reopen_pull_request,delete_tag,delete_branch,mirror_sync_push,mirror_sync_create,mirror_sync_delete,approve_pull_request,reject_pull_request,comment_pull,publish_release,pull_review_dismissed,pull_request_ready_for_review,auto_merge_pull_request
	OpType string `json:"op_type"`
	// The ID of the user who performed the action
	ActUserID int64 `json:"act_user_id"`
	// The user who performed the action
	ActUser *User `json:"act_user"`
	// The ID of the repository associated with the activity
	RepoID int64 `json:"repo_id"`
	// The repository associated with the activity
	Repo *Repository `json:"repo"`
	// The ID of the comment associated with the activity (if applicable)
	CommentID int64 `json:"comment_id"`
	// The comment associated with the activity (if applicable)
	Comment *Comment `json:"comment"`
	// The name of the git reference (branch/tag) associated with the activity
	RefName string `json:"ref_name"`
	// Whether this activity is from a private repository
	IsPrivate bool `json:"is_private"`
	// Additional content or details about the activity
	Content string `json:"content"`
	// The date and time when the activity occurred
	Created time.Time `json:"created"`
}
