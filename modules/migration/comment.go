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

// Comment is a standard comment information
type Comment struct {
	IssueIndex  int64 `yaml:"issue_index"`
	Index       int64
	PosterID    int64  `yaml:"poster_id"`
	PosterName  string `yaml:"poster_name"`
	PosterEmail string `yaml:"poster_email"`
	Created     time.Time
	Updated     time.Time
	Content     string
	Reactions   []*Reaction
}

// GetExternalName ExternalUserMigrated interface
func (c *Comment) GetExternalName() string { return c.PosterName }

// ExternalID ExternalUserMigrated interface
func (c *Comment) GetExternalID() int64 { return c.PosterID }
