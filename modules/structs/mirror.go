// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

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
}
