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
	Inputs map[string]string `json:"inputs,omitempty"`
}

// ActionWorkflow represents a ActionWorkflow
type ActionWorkflow struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	State string `json:"state"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
	HTMLURL   string    `json:"html_url"`
	BadgeURL  string    `json:"badge_url"`
	// swagger:strfmt date-time
	DeletedAt time.Time `json:"deleted_at,omitempty"`
}

// ActionWorkflowResponse returns a ActionWorkflow
type ActionWorkflowResponse struct {
	Workflows  []*ActionWorkflow `json:"workflows"`
	TotalCount int64             `json:"total_count"`
}

// ActionArtifact represents a ActionArtifact
type ActionArtifact struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	SizeInBytes        int64              `json:"size_in_bytes"`
	URL                string             `json:"url"`
	ArchiveDownloadURL string             `json:"archive_download_url"`
	Expired            bool               `json:"expired"`
	WorkflowRun        *ActionWorkflowRun `json:"workflow_run"`

	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	ExpiresAt time.Time `json:"expires_at"`
}

// ActionWorkflowRun represents a WorkflowRun
type ActionWorkflowRun struct {
	ID           int64  `json:"id"`
	RepositoryID int64  `json:"repository_id"`
	HeadSha      string `json:"head_sha"`
}

// ActionArtifactsResponse returns ActionArtifacts
type ActionArtifactsResponse struct {
	Entries    []*ActionArtifact `json:"artifacts"`
	TotalCount int64             `json:"total_count"`
}

// ActionWorkflowStep represents a step of a WorkflowJob
type ActionWorkflowStep struct {
	Name       string `json:"name"`
	Number     int64  `json:"number"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	// swagger:strfmt date-time
	StartedAt time.Time `json:"started_at,omitempty"`
	// swagger:strfmt date-time
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// ActionWorkflowJob represents a WorkflowJob
type ActionWorkflowJob struct {
	ID         int64                 `json:"id"`
	URL        string                `json:"url"`
	HTMLURL    string                `json:"html_url"`
	RunID      int64                 `json:"run_id"`
	RunURL     string                `json:"run_url"`
	Name       string                `json:"name"`
	Labels     []string              `json:"labels"`
	RunAttempt int64                 `json:"run_attempt"`
	HeadSha    string                `json:"head_sha"`
	HeadBranch string                `json:"head_branch,omitempty"`
	Status     string                `json:"status"`
	Conclusion string                `json:"conclusion,omitempty"`
	RunnerID   int64                 `json:"runner_id,omitempty"`
	RunnerName string                `json:"runner_name,omitempty"`
	Steps      []*ActionWorkflowStep `json:"steps"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	StartedAt time.Time `json:"started_at,omitempty"`
	// swagger:strfmt date-time
	CompletedAt time.Time `json:"completed_at,omitempty"`
}
