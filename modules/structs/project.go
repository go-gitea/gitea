// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Project represents a project
// swagger:model
type Project struct {
	ID              int64      `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	OwnerID         int64      `json:"owner_id,omitempty"`
	RepoID          int64      `json:"repo_id,omitempty"`
	CreatorID       int64      `json:"creator_id"`
	IsClosed        bool       `json:"is_closed"`
	TemplateType    int        `json:"template_type"`
	CardType        int        `json:"card_type"`
	Type            int        `json:"type"`
	NumOpenIssues   int64      `json:"num_open_issues,omitempty"`
	NumClosedIssues int64      `json:"num_closed_issues,omitempty"`
	NumIssues       int64      `json:"num_issues,omitempty"`
	// swagger:strfmt date-time
	Created    time.Time  `json:"created"`
	// swagger:strfmt date-time
	Updated    time.Time  `json:"updated"`
	// swagger:strfmt date-time
	ClosedDate *time.Time `json:"closed_date,omitempty"`
	URL        string     `json:"url,omitempty"`
}

// CreateProjectOption represents options for creating a project
// swagger:model
type CreateProjectOption struct {
	// required: true
	Title        string `json:"title" binding:"Required"`
	Description  string `json:"description"`
	TemplateType int    `json:"template_type"`
	CardType     int    `json:"card_type"`
}

// EditProjectOption represents options for editing a project
// swagger:model
type EditProjectOption struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	// Card type: 0=text_only, 1=images_and_text
	CardType *int `json:"card_type,omitempty"`
}

// ProjectColumn represents a project column (board)
// swagger:model
type ProjectColumn struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Default   bool      `json:"default"`
	Sorting   int       `json:"sorting"`
	Color     string    `json:"color,omitempty"`
	ProjectID int64     `json:"project_id"`
	CreatorID int64     `json:"creator_id"`
	NumIssues int64     `json:"num_issues,omitempty"`
	// swagger:strfmt date-time
	Created   time.Time `json:"created"`
	// swagger:strfmt date-time
	Updated   time.Time `json:"updated"`
}

// CreateProjectColumnOption represents options for creating a project column
// swagger:model
type CreateProjectColumnOption struct {
	// required: true
	Title string `json:"title" binding:"Required"`
	Color string `json:"color,omitempty"`
}

// EditProjectColumnOption represents options for editing a project column
// swagger:model
type EditProjectColumnOption struct {
	Title   *string `json:"title,omitempty"`
	Color   *string `json:"color,omitempty"`
	Sorting *int    `json:"sorting,omitempty"`
}


// AddIssueToProjectColumnOption represents options for adding an issue to a project column
// swagger:model
type AddIssueToProjectColumnOption struct {
	// required: true
	IssueIDs []int64 `json:"issue_ids" binding:"Required"`
}

