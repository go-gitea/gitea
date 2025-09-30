// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// AddTimeOption options for adding time to an issue
type AddTimeOption struct {
	// time in seconds
	// required: true
	Time int64 `json:"time" binding:"Required"`
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// username of the user who spent the time working on the issue (optional)
	User string `json:"user_name"`
}

// TrackedTime worked time for an issue / pr
type TrackedTime struct {
	// ID is the unique identifier for the tracked time entry
	ID int64 `json:"id"`
	// swagger:strfmt date-time
	Created time.Time `json:"created"`
	// Time in seconds
	Time int64 `json:"time"`
	// deprecated (only for backwards compatibility)
	UserID int64 `json:"user_id"`
	// username of the user
	UserName string `json:"user_name"`
	// deprecated (only for backwards compatibility)
	IssueID int64 `json:"issue_id"`
	// Issue contains the associated issue information
	Issue *Issue `json:"issue"`
}

// TrackedTimeList represents a list of tracked times
type TrackedTimeList []*TrackedTime
