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
