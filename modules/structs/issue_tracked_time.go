// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// TrackedTimeDeprecated worked time for an issue / pr
type TrackedTimeDeprecated struct {
	ID int64 `json:"id"`
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// Time in seconds
	Time    int64 `json:"time"`
	UserID  int64 `json:"user_id"`
	IssueID int64 `json:"issue_id"`
}

// TrackedTimesDeprecated represent a list of tracked times
type TrackedTimesDeprecated []*TrackedTimeDeprecated

// AddTimeOption options for adding time to an issue
type AddTimeOption struct {
	// time in seconds
	// required: true
	Time int64 `json:"time" binding:"Required"`
}
