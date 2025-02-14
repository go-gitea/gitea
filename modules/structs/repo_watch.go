// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// WatchInfo represents an API watch status of one repository
type WatchInfo struct {
	Subscribed    bool      `json:"subscribed"`
	Ignored       bool      `json:"ignored"`
	Reason        any       `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
	URL           string    `json:"url"`
	RepositoryURL string    `json:"repository_url"`
}
