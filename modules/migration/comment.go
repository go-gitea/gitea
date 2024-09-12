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
	Meta        map[string]any `yaml:"meta,omitempty"` // see models/issues/comment.go for fields in Comment struct
}

// GetExternalName ExternalUserMigrated interface
func (c *Comment) GetExternalName() string { return c.PosterName }

// ExternalID ExternalUserMigrated interface
func (c *Comment) GetExternalID() int64 { return c.PosterID }
