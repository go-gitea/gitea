// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// ProjectMeta represents a project board summary embedded in issue/PR responses
// swagger:model
type ProjectMeta struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       StateType `json:"state"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed   *time.Time `json:"closed_at"`
	ColumnID int64      `json:"column_id"`
	Column   string     `json:"column"`
}
