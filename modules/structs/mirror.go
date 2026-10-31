// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// PushMirrorConfig represents the optional settings of a push mirror
type PushMirrorConfig struct {
	// Do not delete remote branches which do not exist in the local repository
	KeepRemoteBranches bool `json:"keep_remote_branches"`
	// Do not push or delete remote tags
	NoPushTags bool `json:"no_push_tags"`
	// Do not delete remote tags which do not exist in the local repository
	KeepRemoteTags bool `json:"keep_remote_tags"`
	// Only push the branches matching these names or glob patterns, empty means all branches
	BranchFilters []string `json:"branch_filters"`
}

// PushMirrorHistory represents one run of a push mirror
// swagger:model
type PushMirrorHistory struct {
	// The result of the run: success, partial or failed
	Status string `json:"status"`
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// The duration of the run in milliseconds
	DurationMs int64 `json:"duration_ms"`
	// The number of refs which were pushed
	Pushed int `json:"pushed"`
	// The number of remote refs which were deleted
	Deleted int `json:"deleted"`
	// The number of refs which were already up to date
	UpToDate int `json:"up_to_date"`
	// The total number of refs which failed to be pushed
	FailedTotal int `json:"failed_total"`
	// The failed refs, the list may be truncated
	Failed []PushMirrorRefError `json:"failed"`
	// The error message if the whole run failed
	Error string `json:"error"`
}

// PushMirrorRefError represents a ref which failed to be pushed
type PushMirrorRefError struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// CreatePushMirrorOption represents need information to create a push mirror of a repository.
type CreatePushMirrorOption struct {
	// The remote repository URL to push to
	RemoteAddress string `json:"remote_address"`
	// The username for authentication with the remote repository
	RemoteUsername string `json:"remote_username"`
	// The password for authentication with the remote repository
	RemotePassword string `json:"remote_password"`
	// The sync interval for automatic updates
	Interval string `json:"interval"`
	// Whether to sync on every commit
	SyncOnCommit bool `json:"sync_on_commit"`
	// Optional push settings, the default pushes all refs and deletes the remote refs which do not exist locally
	Config *PushMirrorConfig `json:"config"`
}

// PushMirror represents information of a push mirror
// swagger:model
type PushMirror struct {
	// The name of the source repository
	RepoName string `json:"repo_name"`
	// The name of the remote in the git configuration
	RemoteName string `json:"remote_name"`
	// The remote repository URL being mirrored to
	RemoteAddress string `json:"remote_address"`
	// swagger:strfmt date-time
	CreatedUnix time.Time `json:"created"`
	// swagger:strfmt date-time
	LastUpdateUnix *time.Time `json:"last_update"`
	// The last error message encountered during sync
	LastError string `json:"last_error"`
	// The sync interval for automatic updates
	Interval string `json:"interval"`
	// Whether to sync on every commit
	SyncOnCommit bool `json:"sync_on_commit"`
	// The push settings
	Config PushMirrorConfig `json:"config"`
}
