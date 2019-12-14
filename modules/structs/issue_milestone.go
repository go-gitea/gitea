// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Milestone milestone is a collection of issues on one repository
type Milestone struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        StateType `json:"state"`
	OpenIssues   int       `json:"open_issues"`
	ClosedIssues int       `json:"closed_issues"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_on"`
}

// CreateMilestoneOption options for creating a milestone
type CreateMilestoneOption struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_on"`
}

// EditMilestoneOption options for editing a milestone
type EditMilestoneOption struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	State       *string    `json:"state"`
	Deadline    *time.Time `json:"due_on"`
}
