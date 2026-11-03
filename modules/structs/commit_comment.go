// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

type CommitComment struct {
	ID          int64         `json:"id"`
	HTMLURL     string        `json:"html_url"`
	URL         string        `json:"url"`
	Body        string        `json:"body"`
	User        *User         `json:"user"`
	Path        *string       `json:"path"`
	Line        *int64        `json:"line"`
	Position    *int64        `json:"position"`
	Side        string        `json:"side,omitempty"`
	CommitID    string        `json:"commit_id"`
	Attachments []*Attachment `json:"assets"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

type CreateCommitCommentOption struct {
	// required: true
	Body string `json:"body" binding:"Required"`
	Path string `json:"path"`
	// Line is a file line number, not GitHub's diff position, and is required with path.
	Line int64 `json:"line"`
	// Side defaults to new.
	// enum: old,new
	Side string `json:"side" binding:"In(old,new)"`
}

type EditCommitCommentOption struct {
	Body *string `json:"body"`
}
