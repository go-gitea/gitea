// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// StopWatch represent a running stopwatch
type StopWatch struct {
	// swagger:strfmt date-time
	// Created is the time when the stopwatch was started
	Created time.Time `json:"created"`
	// Seconds is the total elapsed time in seconds
	Seconds int64 `json:"seconds"`
	// Duration is a human-readable duration string
	Duration string `json:"duration"`
	// IssueIndex is the index number of the associated issue
	IssueIndex int64 `json:"issue_index"`
	// IssueTitle is the title of the associated issue
	IssueTitle string `json:"issue_title"`
	// RepoOwnerName is the name of the repository owner
	RepoOwnerName string `json:"repo_owner_name"`
	// RepoName is the name of the repository
	RepoName string `json:"repo_name"`
}

// StopWatches represent a list of stopwatches
type StopWatches []StopWatch
