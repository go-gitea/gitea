// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import "time"

// IssueContext is used to map between local and foreign issue/PR ids.
type IssueContext interface {
	LocalID() int64
	ForeignID() int64
}

// BasicIssueContext is a 1:1 mapping between local and foreign ids.
type BasicIssueContext int64

// LocalID gets the local id.
func (c BasicIssueContext) LocalID() int64 {
	return int64(c)
}

// ForeignID gets the foreign id.
func (c BasicIssueContext) ForeignID() int64 {
	return int64(c)
}

// Issue is a standard issue information
type Issue struct {
	Number      int64
	PosterID    int64  `yaml:"poster_id"`
	PosterName  string `yaml:"poster_name"`
	PosterEmail string `yaml:"poster_email"`
	Title       string
	Content     string
	Ref         string
	Milestone   string
	State       string // closed, open
	IsLocked    bool   `yaml:"is_locked"`
	Created     time.Time
	Updated     time.Time
	Closed      *time.Time
	Labels      []*Label
	Reactions   []*Reaction
	Assignees   []string
	Context     IssueContext `yaml:"-"`
}
