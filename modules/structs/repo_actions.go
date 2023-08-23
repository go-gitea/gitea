// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Tag represents a repository tag
type ActionTask struct {
	ID         int64  `json:"id"`
	JobName    string `json:"job_name"`
	WorkflowID string `json:"workflow_id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Commit     string `json:"commit"`
	Duration   string `json:"duration"`
	// swagger:strfmt date-time
	Started time.Time `json:"started,omitempty"`
	// swagger:strfmt date-time
	Stopped time.Time `json:"stopped,omitempty"`
}
