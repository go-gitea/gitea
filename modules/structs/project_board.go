// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

type ProjectBoard struct {
	ID      int64    `json:"id"`
	Title   string   `json:"title"`
	Default bool     `json:"default"`
	Color   string   `json:"color"`
	Sorting int8     `json:"sorting"`
	Project *Project `json:"project"`
	Creator *User    `json:"creator"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// swagger:model
type NewProjectBoardPayload struct {
	// required:true
	Title   string `json:"title"`
	Default bool   `json:"default"`
	Color   string `json:"color"`
	Sorting int8   `json:"sorting`
}

// swagger:model
type UpdateProjectBoardPayload struct {
	Title string `json:"title"`
	Color string `json:"color"`
}
