// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

type Activity struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"user_id"` // Receiver user
	// the type of action
	//
	// enum: create_repo,rename_repo,star_repo,watch_repo,commit_repo,create_issue,create_pull_request,transfer_repo,push_tag,comment_issue,merge_pull_request,close_issue,reopen_issue,close_pull_request,reopen_pull_request,delete_tag,delete_branch,mirror_sync_push,mirror_sync_create,mirror_sync_delete,approve_pull_request,reject_pull_request,comment_pull,publish_release,pull_review_dismissed,pull_request_ready_for_review,auto_merge_pull_request
	OpType    string      `json:"op_type"`
	ActUserID int64       `json:"act_user_id"`
	ActUser   *User       `json:"act_user"`
	RepoID    int64       `json:"repo_id"`
	Repo      *Repository `json:"repo"`
	CommentID int64       `json:"comment_id"`
	Comment   *Comment    `json:"comment"`
	RefName   string      `json:"ref_name"`
	IsPrivate bool        `json:"is_private"`
	Content   string      `json:"content"`
	Created   time.Time   `json:"created"`
}
