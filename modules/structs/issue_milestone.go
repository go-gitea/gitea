// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Milestone milestone is a collection of issues on one repository
type Milestone struct {
	// ID is the unique identifier for the milestone
	ID int64 `json:"id"`
	// Title is the title of the milestone
	Title string `json:"title"`
	// Description provides details about the milestone
	Description string `json:"description"`
	// State indicates if the milestone is open or closed
	State StateType `json:"state"`
	// OpenIssues is the number of open issues in this milestone
	OpenIssues int `json:"open_issues"`
	// ClosedIssues is the number of closed issues in this milestone
	ClosedIssues int `json:"closed_issues"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_on"`
}

// CreateMilestoneOption options for creating a milestone
type CreateMilestoneOption struct {
	// Title is the title of the new milestone
	Title string `json:"title"`
	// Description provides details about the milestone
	Description string `json:"description"`
	// swagger:strfmt date-time
	// Deadline is the due date for the milestone
	Deadline *time.Time `json:"due_on"`
	// enum: open,closed
	// State indicates the initial state of the milestone
	State string `json:"state"`
}

// EditMilestoneOption options for editing a milestone
type EditMilestoneOption struct {
	// Title is the updated title of the milestone
	Title string `json:"title"`
	// Description provides updated details about the milestone
	Description *string `json:"description"`
	// State indicates the updated state of the milestone
	State *string `json:"state"`
	// Deadline is the updated due date for the milestone
	Deadline *time.Time `json:"due_on"`
}
