// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// UserGroup represents a user group in API responses.
type UserGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID int64  `json:"parent_id"`
}

// CreateUserGroupOption contains options for creating a user group.
type CreateUserGroupOption struct {
	Name     string `json:"name" binding:"Required"`
	ParentID int64  `json:"parent_id"`
}

// EditUserGroupOption contains options for editing a user group.
type EditUserGroupOption struct {
	Name     string `json:"name" binding:"Required"`
	ParentID int64  `json:"parent_id"`
}

// UserGroupMembersOption contains the member list for a user group.
type UserGroupMembersOption struct {
	UserIDs []int64 `json:"user_ids" binding:"Required"`
}
