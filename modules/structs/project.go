// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// NewProjectOption options when creating a new project
// swagger:model
type NewProjectOption struct {
	// required:true
	// Keep compatibility with Github API to use "name" instead of "title"
	Name string `json:"name" binding:"Required"`
	// required:true
	// enum: , BasicKanban, BugTriage
	// Note: this is the same as TemplateType in models/project/template.go
	TemplateType string `json:"template_type"`
	// required:true
	// enum: TextOnly, ImagesAndText
	CardType string `json:"card_type"`
	// Keep compatibility with Github API to use "body" instead of "description"
	Body string `json:"body"`
}

// UpdateProjectOption options when updating a project
// swagger:model
type UpdateProjectOption struct {
	// required:true
	// Keep compatibility with Github API to use "name" instead of "title"
	Name string `json:"name" binding:"Required"`
	// Keep compatibility with Github API to use "body" instead of "description"
	Body string `json:"body"`
}

// Project represents a project
// swagger:model
type Project struct {
	ID int64 `json:"id"`
	// Keep compatibility with Github API to use "name" instead of "title"
	Name string `json:"name"`
	// Keep compatibility with Github API to use "body" instead of "description"
	Body string `json:"body"`
	// required:true
	// enum: , BasicKanban, BugTriage
	// Note: this is the same as TemplateType in models/project/template.go
	TemplateType string `json:"template_type"`
	// enum: open, closed
	State string `json:"state"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`

	Repo    *RepositoryMeta `json:"repository"`
	Creator *User           `json:"creator"`
	Owner   *User           `json:"owner"`
}
