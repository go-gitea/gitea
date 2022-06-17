// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import "time"

// Issue is a standard issue information
type Issue struct {
	Number       int64             `json:"number"`
	PosterID     int64             `yaml:"poster_id" json:"poster_id"`
	PosterName   string            `yaml:"poster_name" json:"poster_name"`
	PosterEmail  string            `yaml:"poster_email" json:"poster_email"`
	Title        string            `json:"title"`
	Content      string            `json:"content"`
	Ref          string            `json:"ref"`
	Milestone    string            `json:"milestone"`
	State        string            `json:"state"` // closed, open
	IsLocked     bool              `yaml:"is_locked" json:"is_locked"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
	Closed       *time.Time        `json:"closed"`
	Labels       []*Label          `json:"labels"`
	Reactions    []*Reaction       `json:"reactions"`
	Assignees    []string          `json:"assignees"`
	ForeignIndex int64             `json:"foreign_id"`
	Context      DownloaderContext `yaml:"-"`
}

// GetExternalName ExternalUserMigrated interface
func (i *Issue) GetExternalName() string { return i.PosterName }

// GetExternalID ExternalUserMigrated interface
func (i *Issue) GetExternalID() int64 { return i.PosterID }

func (i *Issue) GetLocalIndex() int64          { return i.Number }
func (i *Issue) GetForeignIndex() int64        { return i.ForeignIndex }
func (i *Issue) GetContext() DownloaderContext { return i.Context }
