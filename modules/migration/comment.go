// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "time"

// Commentable can be commented upon
type Commentable interface {
	Reviewable
}

// CommentedEntity is an issue or pull request with comments updated in a time range, found through its comments
// because a new comment does not always move the entity itself in its host's updated order
type CommentedEntity struct {
	Number int64
	IsPull bool
	Since  time.Time // oldest new comment
	Until  time.Time // newest new comment
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
