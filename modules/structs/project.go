// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Project represents a project.
//
// Gitea projects can only contain issues — note cards and pull requests are
// not modeled as project items.
//
// swagger:model
type Project struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	OwnerID     int64     `json:"owner_id,omitempty"`
	RepoID      int64     `json:"repo_id,omitempty"`
	Creator     *User     `json:"creator,omitempty"`
	State       StateType `json:"state"`
	// Template type: "none", "basic_kanban" or "bug_triage"
	TemplateType string `json:"template_type"`
	// Card type: "text_only" or "images_and_text"
	CardType string `json:"card_type"`
	// Project type: "individual", "repository" or "organization"
	Type            string `json:"type"`
	NumOpenIssues   int64  `json:"num_open_issues,omitempty"`
	NumClosedIssues int64  `json:"num_closed_issues,omitempty"`
	NumIssues       int64  `json:"num_issues,omitempty"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
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
	Title       *string `json:"title,omitempty"`
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
	NumIssues int64  `json:"num_issues,omitempty"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
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
	Title *string `json:"title,omitempty"`
	// Column color in 6-digit hex format, e.g. #FF0000
	Color   *string `json:"color,omitempty"`
	Sorting *int    `json:"sorting,omitempty"`
}

// MoveProjectIssueOption represents options for moving an issue between columns
// swagger:model
type MoveProjectIssueOption struct {
	// Target column to move the issue into
	// required: true
	ColumnID int64 `json:"column_id" binding:"Required"`
	// Optional sorting position within the target column
	Sorting *int64 `json:"sorting,omitempty"`
}
