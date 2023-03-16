// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

type Activity struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"` // Receiver user
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
