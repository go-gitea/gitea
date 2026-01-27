// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Label a label to an issue or a pr
// swagger:model
type Label struct {
	// ID is the unique identifier for the label
	ID int64 `json:"id"`
	// Name is the display name of the label
	Name string `json:"name"`
	// example: false
	Exclusive bool `json:"exclusive"`
	// example: false
	IsArchived bool `json:"is_archived"`
	// example: 00aabb
	Color string `json:"color"`
	// Description provides additional context about the label's purpose
	Description string `json:"description"`
	// URL is the API endpoint for accessing this label
	URL string `json:"url"`
}

// CreateLabelOption options for creating a label
type CreateLabelOption struct {
	// required:true
	// Name is the display name for the new label
	Name string `json:"name" binding:"Required"`
	// example: false
	Exclusive bool `json:"exclusive"`
	// required:true
	// example: #00aabb
	Color string `json:"color" binding:"Required"`
	// Description provides additional context about the label's purpose
	Description string `json:"description"`
	// example: false
	IsArchived bool `json:"is_archived"`
}

// EditLabelOption options for editing a label
type EditLabelOption struct {
	// Name is the new display name for the label
	Name *string `json:"name"`
	// example: false
	Exclusive *bool `json:"exclusive"`
	// example: #00aabb
	Color *string `json:"color"`
	// Description provides additional context about the label's purpose
	Description *string `json:"description"`
	// example: false
	IsArchived *bool `json:"is_archived"`
}

// IssueLabelsOption a collection of labels
type IssueLabelsOption struct {
	// Labels can be a list of integers representing label IDs
	// or a list of strings representing label names
	Labels []any `json:"labels"`
}

// LabelTemplate info of a Label template
type LabelTemplate struct {
	// Name is the display name of the label template
	Name string `json:"name"`
	// example: false
	Exclusive bool `json:"exclusive"`
	// example: 00aabb
	Color string `json:"color"`
	// Description provides additional context about the label template's purpose
	Description string `json:"description"`
}
