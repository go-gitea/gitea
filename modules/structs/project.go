// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Project represents a project
type Project struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	TemplateType uint8  `json:"template_type"`
	CardType     uint8  `json:"card_type"`
	OwnerID      int64  `json:"owner_id"`
	RepoID       int64  `json:"repo_id"`
	CreatorID    int64  `json:"creator_id"`
	IsClosed     bool   `json:"is_closed"`
	Type         uint8  `json:"type"`

	CreatedUnix    int64 `json:"created_unix"`
	UpdatedUnix    int64 `json:"updated_unix"`
	ClosedDateUnix int64 `json:"closed_date_unix"`
}

// CreateProjectOption options for creating a project
type CreateProjectOption struct {
	// required:true
	Title        string `json:"title" binding:"Required;MaxSize(100)"`
	Content      string `json:"content"`
	TemplateType uint8  `json:"template_type"`
	CardType     uint8  `json:"card_type"`
}

// EditProjectOption options for editing a project
type EditProjectOption struct {
	Title    string `json:"title" binding:"MaxSize(100)"`
	Content  string `json:"content"`
	CardType uint8  `json:"card_type"`
}

// MoveColumnsOption options for moving columns
type MovedColumnsOption struct {
	Columns []struct {
		ColumnID int64 `json:"columnID"`
		Sorting  int64 `json:"sorting"`
	} `json:"columns"`
}
