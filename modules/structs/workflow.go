// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ref https://docs.github.com/en/webhooks/webhook-events-and-payloads#workflow_run

type Workflow struct {
	BadgeURL  string `json:"badge_url"`
	CreatedAt string `json:"created_at"`
	HTMLURL   string `json:"html_url"`
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
	URL       string `json:"url"`
}

type WorkflowRun struct {
	Actor           *User          `json:"actor"`
	ArtifactsURL    string         `json:"artifacts_url"`
	CancelURL       string         `json:"cancel_url"`
	Conclusion      string         `json:"conclusion"` // Can be one of: success, failure, neutral, cancelled, timed_out, action_required, stale, null, skipped
	CreatedAt       string         `json:"created_at"`
	Event           string         `json:"event"`
	HeadBranch      string         `json:"head_branch"`
	HeadCommit      *Commit        `json:"head_commit"`
	HeadRepository  *Repository    `json:"head_repository"`
	HeadSHA         string         `json:"head_sha"`
	HTMLURL         string         `json:"html_url"`
	ID              int64          `json:"id"`
	JobsURL         string         `json:"jobs_url"`
	LogsURL         string         `json:"logs_url"`
	Name            string         `json:"name"`
	Path            string         `json:"path"`
	PullRequests    []*PullRequest `json:"pull_requests"`
	Repository      *Repository    `json:"repository"`
	ReRunURL        string         `json:"rerun_url"`
	RunNumber       int64          `json:"run_number"`
	RunStartedAt    string         `json:"run_started_at"`
	Status          string         `json:"status"` // Can be one of: requested, in_progress, completed, queued, pending, waiting
	TriggeringActor *User          `json:"triggering_actor"`
	UpdatedAt       string         `json:"updated_at"`
	URL             string         `json:"url"`
	WorkflowID      int64          `json:"workflow_id"`
	WorkflowURL     string         `json:"workflow_url"`
}
