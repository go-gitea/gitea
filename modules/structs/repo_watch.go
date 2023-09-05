// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// RepoCustomWatchOptions options when watching custom events of a repo
type RepoCustomWatchOptions struct {
	Issues       bool `json:"issues"`
	PullRequests bool `json:"pull_requests"`
	Releases     bool `json:"releases"`
}

// WatchInfo represents an API watch status of one repository
type WatchInfo struct {
	Subscribed         bool                   `json:"subscribed"`
	Ignored            bool                   `json:"ignored"`
	IsCustom           bool                   `json:"is_custom"`
	Reason             any                    `json:"reason"`
	CreatedAt          time.Time              `json:"created_at"`
	URL                string                 `json:"url"`
	RepositoryURL      string                 `json:"repository_url"`
	CustomWatchOptions RepoCustomWatchOptions `json:"custom_watch_options"`
}
