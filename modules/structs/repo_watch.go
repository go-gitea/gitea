// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// WatchInfo represents an API watch status of one repository
type WatchInfo struct {
	// Whether the repository is being watched for notifications
	Subscribed bool `json:"subscribed"`
	// Whether notifications for the repository are ignored
	Ignored bool `json:"ignored"`
	// The reason for the current watch status
	Reason any `json:"reason"`
	// The timestamp when the watch status was created
	CreatedAt time.Time `json:"created_at"`
	// The URL for managing the watch status
	URL string `json:"url"`
	// The URL of the repository being watched
	RepositoryURL string `json:"repository_url"`
}
