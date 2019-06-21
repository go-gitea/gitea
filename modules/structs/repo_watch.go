// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// WatchInfo represents an API watch status of one repository
type WatchInfo struct {
	Subscribed    bool        `json:"subscribed"`
	Ignored       bool        `json:"ignored"`
	Reason        interface{} `json:"reason"`
	CreatedAt     time.Time   `json:"created_at"`
	URL           string      `json:"url"`
	RepositoryURL string      `json:"repository_url"`
}
