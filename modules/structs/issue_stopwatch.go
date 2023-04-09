// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// StopWatch represent a running stopwatch
type StopWatch struct {
	// swagger:strfmt date-time
	Created       time.Time `json:"created"`
	Seconds       int64     `json:"seconds"`
	Duration      string    `json:"duration"`
	IssueIndex    int64     `json:"issue_index"`
	IssueTitle    string    `json:"issue_title"`
	RepoOwnerName string    `json:"repo_owner_name"`
	RepoName      string    `json:"repo_name"`
}

// StopWatches represent a list of stopwatches
type StopWatches []StopWatch
