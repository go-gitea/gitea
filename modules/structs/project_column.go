// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Column represents a project column
type Column struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Color string `json:"color"`
}

// EditProjectColumnOption options for editing a project column
type EditProjectColumnOption struct {
	Title   string `json:"title" binding:"MaxSize(100)"`
	Sorting int8   `json:"sorting"`
	Color   string `json:"color" binding:"MaxSize(7)"`
}

// CreateProjectColumnOption options for creating a project column
type CreateProjectColumnOption struct {
	// required:true
	Title   string `json:"title" binding:"Required;MaxSize(100)"`
	Sorting int8   `json:"sorting"`
	Color   string `json:"color" binding:"MaxSize(7)"`
}
