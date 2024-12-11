// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// ActionTask represents a ActionTask
type ActionTask struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	HeadBranch   string `json:"head_branch"`
	HeadSHA      string `json:"head_sha"`
	RunNumber    int64  `json:"run_number"`
	Event        string `json:"event"`
	DisplayTitle string `json:"display_title"`
	Status       string `json:"status"`
	WorkflowID   string `json:"workflow_id"`
	URL          string `json:"url"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	RunStartedAt time.Time `json:"run_started_at"`
}

// ActionTaskResponse returns a ActionTask
type ActionTaskResponse struct {
	Entries    []*ActionTask `json:"workflow_runs"`
	TotalCount int64         `json:"total_count"`
}

// CreateActionWorkflowDispatch represents the payload for triggering a workflow dispatch event
// swagger:model
type CreateActionWorkflowDispatch struct {
	// required: true
	// example: refs/heads/main
	Ref string `json:"ref" binding:"Required"`
	// required: false
	Inputs map[string]any `json:"inputs,omitempty"`
}

// ActionWorkflow represents a ActionWorkflow
type ActionWorkflow struct {
	ID     string `json:"id"`
	NodeID string `json:"node_id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	State  string `json:"state"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
	HTMLURL   string    `json:"html_url"`
	BadgeURL  string    `json:"badge_url"`
	// swagger:strfmt date-time
	DeletedAt time.Time `json:"deleted_at"`
}

// ActionWorkflowResponse returns a ActionWorkflow
type ActionWorkflowResponse struct {
	Workflows  []*ActionWorkflow `json:"workflows"`
	TotalCount int64             `json:"total_count"`
}
