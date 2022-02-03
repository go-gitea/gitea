// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import "time"

// Commentable can be commented upon
type Commentable interface {
	GetLocalIndex() int64
	GetForeignIndex() int64
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
	Type        string // empty means comment
	IssueIndex  int64  `yaml:"issue_index"`
	Index       int64
	PosterID    int64  `yaml:"poster_id"`
	PosterName  string `yaml:"poster_name"`
	PosterEmail string `yaml:"poster_email"`
	Created     time.Time
	Updated     time.Time
	Content     string
	Reactions   []*Reaction
	Assets      []*Asset
}

// GetExternalName ExternalUserMigrated interface
func (c *Comment) GetExternalName() string { return c.PosterName }

// ExternalID ExternalUserMigrated interface
func (c *Comment) GetExternalID() int64 { return c.PosterID }
