// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// swagger:model
type NewProjectPayload struct {
	// required:true
	Title string `json:"title"       binding:"Required"`
	// required:true
	BoardType uint8 `json:"board_type"`
	// required:true
	CardType    uint8  `json:"card_type"`
	Description string `json:"description"`
}

// swagger:model
type UpdateProjectPayload struct {
	// required:true
	Title       string `json:"title"       binding:"Required"`
	Description string `json:"description"`
}

// swagger:model
type Project struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	TemplateType uint8  `json:"board_type"`
	IsClosed     bool   `json:"is_closed"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed time.Time `json:"closed_at"`

	Repo    *RepositoryMeta `json:"repository"`
	Creator *User           `json:"creator"`
	Owner   *User           `json:"owner"`
}
