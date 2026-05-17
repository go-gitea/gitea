// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ProjectWorkflowRule represents a workflow filter or action item.
// swagger:model
type ProjectWorkflowRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ProjectWorkflowCapabilities describes which filters and actions an event supports.
// swagger:model
type ProjectWorkflowCapabilities struct {
	AvailableFilters []string `json:"available_filters"`
	AvailableActions []string `json:"available_actions"`
}

// ProjectWorkflow represents a project workflow.
// swagger:model
type ProjectWorkflow struct {
	ID            int64                       `json:"id"`
	EventID       string                      `json:"event_id"`
	DisplayName   string                      `json:"display_name"`
	WorkflowEvent string                      `json:"workflow_event"`
	Capabilities  ProjectWorkflowCapabilities `json:"capabilities"`
	Filters       []ProjectWorkflowRule       `json:"filters"`
	Actions       []ProjectWorkflowRule       `json:"actions"`
	Summary       string                      `json:"summary"`
	Enabled       bool                        `json:"enabled"`
	IsConfigured  bool                        `json:"is_configured"`
}

// ProjectWorkflowColumnOption represents a selectable project column.
// swagger:model
type ProjectWorkflowColumnOption struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Color string `json:"color"`
}

// ProjectWorkflowLabelOption represents a selectable project label.
// swagger:model
type ProjectWorkflowLabelOption struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Color          string `json:"color"`
	Description    string `json:"description"`
	Exclusive      bool   `json:"exclusive"`
	ExclusiveOrder int    `json:"exclusive_order"`
}

// ProjectWorkflowOptions represents the project workflow configuration options.
// swagger:model
type ProjectWorkflowOptions struct {
	Columns []*ProjectWorkflowColumnOption `json:"columns"`
	Labels  []*ProjectWorkflowLabelOption  `json:"labels"`
}

// ProjectWorkflowFilterOptions represents editable workflow filters.
// swagger:model
type ProjectWorkflowFilterOptions struct {
	IssueType    string   `json:"issue_type,omitempty"`
	SourceColumn string   `json:"source_column,omitempty"`
	TargetColumn string   `json:"target_column,omitempty"`
	Labels       []string `json:"labels,omitempty"`
}

// ProjectWorkflowActionOptions represents editable workflow actions.
// swagger:model
type ProjectWorkflowActionOptions struct {
	Column       string   `json:"column,omitempty"`
	AddLabels    []string `json:"add_labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
	IssueState   string   `json:"issue_state,omitempty"`
}

// CreateProjectWorkflowOption represents the payload for creating a project workflow.
// swagger:model
type CreateProjectWorkflowOption struct {
	EventID string                       `json:"event_id" binding:"Required"`
	Filters ProjectWorkflowFilterOptions `json:"filters"`
	Actions ProjectWorkflowActionOptions `json:"actions"`
}

// EditProjectWorkflowOption represents the payload for editing a project workflow.
// swagger:model
type EditProjectWorkflowOption struct {
	Filters ProjectWorkflowFilterOptions `json:"filters"`
	Actions ProjectWorkflowActionOptions `json:"actions"`
}
