// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// StopWatch represent a running stopwatch
type StopWatch struct {
	// swagger:strfmt date-time
	Created    time.Time `json:"created"`
	IssueIndex int64     `json:"issue_index"`
}

// StopWatches represent a list of stopwatches
type StopWatches []StopWatch
