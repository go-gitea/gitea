// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// ProjectWorkflowFilters restricts which items a project workflow runs for
// swagger:model
type ProjectWorkflowFilters struct {
	// "issue" or "pull_request", empty matches both
	IssueType string `json:"issue_type,omitempty"`
	// only for item_column_changed
	SourceColumnID int64 `json:"source_column_id,omitempty"`
	// only for item_column_changed
	TargetColumnID int64 `json:"target_column_id,omitempty"`
	// the item must have all of these labels
	LabelIDs []int64 `json:"label_ids,omitempty"`
}

// ProjectWorkflowActions describes what a project workflow does to a matching item
// swagger:model
type ProjectWorkflowActions struct {
	ColumnID       int64   `json:"column_id,omitempty"`
	AddLabelIDs    []int64 `json:"add_label_ids,omitempty"`
	RemoveLabelIDs []int64 `json:"remove_label_ids,omitempty"`
	// "close" or "reopen"
	IssueState string `json:"issue_state,omitempty"`
}

// ProjectWorkflow represents a project workflow
// swagger:model
type ProjectWorkflow struct {
	ID int64 `json:"id"`
	// one of item_opened, item_added_to_project, item_removed_from_project, item_reopened, item_closed,
	// item_column_changed, code_changes_requested, code_review_approved, pull_request_merged
	Event   string                 `json:"event"`
	Enabled bool                   `json:"enabled"`
	Filters ProjectWorkflowFilters `json:"filters"`
	Actions ProjectWorkflowActions `json:"actions"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateProjectWorkflowOption options for creating a project workflow
// swagger:model
type CreateProjectWorkflowOption struct {
	// required: true
	Event string `json:"event" binding:"Required"`
	// defaults to true
	Enabled *bool                  `json:"enabled,omitempty"`
	Filters ProjectWorkflowFilters `json:"filters"`
	Actions ProjectWorkflowActions `json:"actions"`
}

// EditProjectWorkflowOption options for editing a project workflow, omitted fields are left unchanged
// swagger:model
type EditProjectWorkflowOption struct {
	Enabled *bool                   `json:"enabled,omitempty"`
	Filters *ProjectWorkflowFilters `json:"filters,omitempty"`
	Actions *ProjectWorkflowActions `json:"actions,omitempty"`
}
