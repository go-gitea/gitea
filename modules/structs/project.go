// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Project represents a project
// swagger:model
type Project struct {
	// Unique identifier of the project
	ID int64 `json:"id"`
	// Project title
	Title string `json:"title"`
	// Project description
	Description string `json:"description"`
	// Owner ID (for organization or user projects)
	OwnerID int64 `json:"owner_id,omitempty"`
	// Repository ID (for repository projects)
	RepoID int64 `json:"repo_id,omitempty"`
	// Creator ID
	CreatorID int64 `json:"creator_id"`
	// Whether the project is closed
	IsClosed bool `json:"is_closed"`
	// Template type: 0=none, 1=basic_kanban, 2=bug_triage
	TemplateType int `json:"template_type"`
	// Card type: 0=text_only, 1=images_and_text
	CardType int `json:"card_type"`
	// Project type: 1=individual, 2=repository, 3=organization
	Type int `json:"type"`
	// Number of open issues
	NumOpenIssues int64 `json:"num_open_issues,omitempty"`
	// Number of closed issues
	NumClosedIssues int64 `json:"num_closed_issues,omitempty"`
	// Total number of issues
	NumIssues int64 `json:"num_issues,omitempty"`
	// Created time
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// Updated time
	// swagger:strfmt date-time
	Updated time.Time `json:"updated"`
	// Closed time
	// swagger:strfmt date-time
	ClosedDate *time.Time `json:"closed_date,omitempty"`
	// Project URL
	URL string `json:"url,omitempty"`
}

// CreateProjectOption represents options for creating a project
// swagger:model
type CreateProjectOption struct {
	// required: true
	Title string `json:"title" binding:"Required"`
	// Project description
	Description string `json:"description"`
	// Template type: 0=none, 1=basic_kanban, 2=bug_triage
	TemplateType int `json:"template_type"`
	// Card type: 0=text_only, 1=images_and_text
	CardType int `json:"card_type"`
}

// EditProjectOption represents options for editing a project
// swagger:model
type EditProjectOption struct {
	// Project title
	Title *string `json:"title,omitempty"`
	// Project description
	Description *string `json:"description,omitempty"`
	// Card type: 0=text_only, 1=images_and_text
	CardType *int `json:"card_type,omitempty"`
	// Whether the project is closed
	IsClosed *bool `json:"is_closed,omitempty"`
}

// ProjectColumn represents a project column (board)
// swagger:model
type ProjectColumn struct {
	// Unique identifier of the column
	ID int64 `json:"id"`
	// Column title
	Title string `json:"title"`
	// Whether this is the default column
	Default bool `json:"default"`
	// Sorting order
	Sorting int `json:"sorting"`
	// Column color (hex format)
	Color string `json:"color,omitempty"`
	// Project ID
	ProjectID int64 `json:"project_id"`
	// Creator ID
	CreatorID int64 `json:"creator_id"`
	// Number of issues in this column
	NumIssues int64 `json:"num_issues,omitempty"`
	// Created time
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// Updated time
	// swagger:strfmt date-time
	Updated time.Time `json:"updated"`
}

// CreateProjectColumnOption represents options for creating a project column
// swagger:model
type CreateProjectColumnOption struct {
	// required: true
	Title string `json:"title" binding:"Required"`
	// Column color (hex format, e.g., #FF0000)
	Color string `json:"color,omitempty"`
}

// EditProjectColumnOption represents options for editing a project column
// swagger:model
type EditProjectColumnOption struct {
	// Column title
	Title *string `json:"title,omitempty"`
	// Column color (hex format)
	Color *string `json:"color,omitempty"`
	// Sorting order
	Sorting *int `json:"sorting,omitempty"`
}

// MoveProjectColumnOption represents options for moving a project column
// swagger:model
type MoveProjectColumnOption struct {
	// Position to move the column to (0-based index)
	// required: true
	Position int `json:"position" binding:"Required"`
}

// AddIssueToProjectColumnOption represents options for adding an issue to a project column
// swagger:model
type AddIssueToProjectColumnOption struct {
	// Issue ID to add to the column
	// required: true
	IssueID int64 `json:"issue_id" binding:"Required"`
}
