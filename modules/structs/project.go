// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Project represents a project
// swagger:model
type Project struct {
	// ID is the unique identifier for the project
	ID int64 `json:"id"`
	// Title is the title of the project
	Title string `json:"title"`
	// Description provides details about the project
	Description string `json:"description"`
	// OwnerID is the owner of the project (for org-level projects)
	OwnerID int64 `json:"owner_id,omitempty"`
	// RepoID is the repository this project belongs to (for repo-level projects)
	RepoID int64 `json:"repo_id,omitempty"`
	// CreatorID is the user who created the project
	CreatorID int64 `json:"creator_id"`
	// IsClosed indicates if the project is closed
	IsClosed bool `json:"is_closed"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at,omitempty"`
}
