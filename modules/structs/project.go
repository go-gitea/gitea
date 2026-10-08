// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Project represents a project.
//
// Projects track issues and pull requests, standalone note cards are not supported.
//
// swagger:model
type Project struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	OwnerID     int64  `json:"owner_id"`
	RepoID      int64  `json:"repo_id"`
	Creator     *User  `json:"creator,omitempty"`
	// Deprecated: use Creator instead
	CreatorID int64     `json:"creator_id"`
	State     StateType `json:"state"`
	// Deprecated: use State instead
	IsClosed bool `json:"is_closed"`
	// Template type: "none", "basic_kanban" or "bug_triage"
	TemplateType string `json:"template_type"`
	// Card type: "text_only" or "images_and_text"
	CardType string `json:"card_type"`
	// Project type: "individual", "repository" or "organization"
	Type            string `json:"type"`
	NumOpenIssues   int64  `json:"num_open_issues"`
	NumClosedIssues int64  `json:"num_closed_issues"`
	NumIssues       int64  `json:"num_issues"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// null only for legacy rows that carry no update timestamp
	// swagger:strfmt date-time
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// swagger:strfmt date-time
	ClosedAt *time.Time `json:"closed_at,omitempty"`
	HTMLURL  string     `json:"html_url,omitempty"`
}

// CreateProjectOption represents options for creating a project
// swagger:model
type CreateProjectOption struct {
	// required: true
	Title       string `json:"title" binding:"Required"`
	Description string `json:"description"`
	// Template type: "none", "basic_kanban" or "bug_triage"
	TemplateType string `json:"template_type"`
	// Card type: "text_only" or "images_and_text"
	CardType string `json:"card_type"`
}

// EditProjectOption represents options for editing a project
// swagger:model
type EditProjectOption struct {
	Title       *string `json:"title,omitempty" binding:"TrimSpace;Required"`
	Description *string `json:"description,omitempty"`
	// Card type: "text_only" or "images_and_text"
	CardType *string    `json:"card_type,omitempty"`
	State    *StateType `json:"state,omitempty"`
}

// ProjectColumn represents a project column (board)
// swagger:model
type ProjectColumn struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Default   bool   `json:"default"`
	Sorting   int    `json:"sorting"`
	Color     string `json:"color,omitempty"`
	ProjectID int64  `json:"project_id"`
	Creator   *User  `json:"creator,omitempty"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// null only for legacy rows that carry no update timestamp
	// swagger:strfmt date-time
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// CreateProjectColumnOption represents options for creating a project column
// swagger:model
type CreateProjectColumnOption struct {
	// required: true
	Title string `json:"title" binding:"Required"`
	// Column color in 6-digit hex format, e.g. #FF0000
	Color string `json:"color,omitempty"`
}

// EditProjectColumnOption represents options for editing a project column
// swagger:model
type EditProjectColumnOption struct {
	Title *string `json:"title,omitempty" binding:"TrimSpace;Required"`
	// Column color in 6-digit hex format, e.g. #FF0000
	Color *string `json:"color,omitempty"`
	// Position of the column within the project, between -128 and 127
	Sorting *int `json:"sorting,omitempty"`
}

// MoveProjectColumnsOption represents options for reordering a project's columns
// swagger:model
type MoveProjectColumnsOption struct {
	// Every column ID of the project, in the desired left-to-right order
	// required: true
	ColumnIDs []int64 `json:"column_ids" binding:"Required"`
}

// MoveProjectIssueOption represents options for moving an issue between columns
// swagger:model
type MoveProjectIssueOption struct {
	// Target column to move the issue into
	// required: true
	ColumnID int64 `json:"column_id" binding:"Required"`
	// Position within the column, ascending. Omit to append. Negative values sort above
	// the rest, equal values are ordered newest first.
	Sorting *int64 `json:"sorting,omitempty"`
}
