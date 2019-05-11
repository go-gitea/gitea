// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Label a label to an issue or a pr
// swagger:model
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// example: 00aabb
	Color string `json:"color"`
	URL   string `json:"url"`
}

// CreateLabelOption options for creating a label
type CreateLabelOption struct {
	// required:true
	Name string `json:"name" binding:"Required"`
	// required:true
	// example: #00aabb
	Color string `json:"color" binding:"Required;Size(7)"`
}

// EditLabelOption options for editing a label
type EditLabelOption struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// IssueLabelsOption a collection of labels
type IssueLabelsOption struct {
	// list of label IDs
	Labels []int64 `json:"labels"`
}
