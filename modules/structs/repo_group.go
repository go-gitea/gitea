// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Group represents a group of repositories and subgroups in an organization
type Group struct {
	ID            int64  `json:"id"`
	Owner         *User  `json:"owner"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ParentGroupID int64  `json:"parentGroupID"`
	NumRepos      int64  `json:"num_repos"`
	NumSubgroups  int64  `json:"num_subgroups"`
	Link          string `json:"link"`
	SortOrder     int    `json:"sort_order"`
	AvatarURL     string `json:"avatar_url"`
}

// NewGroupOption represents options for creating a new group in an organization
// swagger:model
type NewGroupOption struct {
	// the name for the newly created group
	//
	// required: true
	Name string `json:"name" binding:"Required"`
	// the description of the newly created group
	Description string `json:"description"`
	// the visibility of the newly created group
	Visibility VisibleType `json:"visibility"`
}

// MoveGroupOption represents options for changing a group or repo's parent and sort order
// swagger:model
type MoveGroupOption struct {
	// the new parent group. can be 0 to specify no parent
	//
	// required: true
	NewParent int64 `json:"newParent" binding:"Required"`
	// the position of this group in its new parent
	NewPos *int `json:"newPos,omitempty"`
}

// EditGroupOption represents options for editing a repository group
// swagger:model
type EditGroupOption struct {
	// the new name of the group
	Name *string `json:"name,omitempty"`
	// the new description of the group
	Description *string `json:"description,omitempty"`
	// the new visibility of the group
	Visibility *VisibleType `json:"visibility,omitempty"`
}
