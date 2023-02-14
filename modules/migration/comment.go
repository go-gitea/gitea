// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "time"

// Commentable can be commented upon
type Commentable interface {
	Reviewable
	GetContext() DownloaderContext
}

/*
"comment",
	"reopen",
	"close",
	"issue_ref",
	"commit_ref",
	"comment_ref",
	"pull_ref",
	"label",
	"milestone",
	"assignees",
	"change_title",
	"delete_branch",
	"start_tracking",
	"stop_tracking",
	"add_time_manual",
	"cancel_tracking",
	"added_deadline",
	"modified_deadline",
	"removed_deadline",
	"add_dependency",
	"remove_dependency",
	"code",
	"review",
	"lock",
	"unlock",
	"change_target_branch",
	"delete_time_manual",
	"review_request",
	"merge_pull",
	"pull_push",
	"project",
	"project_board",
	"dismiss_review",
	"change_issue_ref",
*/

// Comment is a standard comment information
type Comment struct {
	IssueIndex  int64 `yaml:"issue_index"`
	Index       int64
	CommentType string `yaml:"comment_type"` // see `commentStrings` in models/issues/comment.go
	PosterID    int64  `yaml:"poster_id"`
	PosterName  string `yaml:"poster_name"`
	PosterEmail string `yaml:"poster_email"`
	Created     time.Time
	Updated     time.Time
	Content     string
	Reactions   []*Reaction
	Meta        map[string]interface{} `yaml:"meta,omitempty"` // see models/issues/comment.go for fields in Comment struct
	Assets      []*Asset
}

// GetExternalName ExternalUserMigrated interface
func (c *Comment) GetExternalName() string { return c.PosterName }

// ExternalID ExternalUserMigrated interface
func (c *Comment) GetExternalID() int64 { return c.PosterID }
