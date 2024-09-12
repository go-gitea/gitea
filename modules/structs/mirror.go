// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// CreatePushMirrorOption represents need information to create a push mirror of a repository.
type CreatePushMirrorOption struct {
	RemoteAddress  string `json:"remote_address"`
	RemoteUsername string `json:"remote_username"`
	RemotePassword string `json:"remote_password"`
	Interval       string `json:"interval"`
	SyncOnCommit   bool   `json:"sync_on_commit"`
}

// PushMirror represents information of a push mirror
// swagger:model
type PushMirror struct {
	RepoName      string `json:"repo_name"`
	RemoteName    string `json:"remote_name"`
	RemoteAddress string `json:"remote_address"`
	// swagger:strfmt date-time
	CreatedUnix time.Time `json:"created"`
	// swagger:strfmt date-time
	LastUpdateUnix *time.Time `json:"last_update"`
	LastError      string     `json:"last_error"`
	Interval       string     `json:"interval"`
	SyncOnCommit   bool       `json:"sync_on_commit"`
}
