// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Organization represents an organization
type Organization struct {
	// The unique identifier of the organization
	ID int64 `json:"id"`
	// The name of the organization
	Name string `json:"name"`
	// The full display name of the organization
	FullName string `json:"full_name"`
	// The email address of the organization
	Email string `json:"email"`
	// The URL of the organization's avatar
	AvatarURL string `json:"avatar_url"`
	// The description of the organization
	Description string `json:"description"`
	// The website URL of the organization
	Website string `json:"website"`
	// The location of the organization
	Location string `json:"location"`
	// The visibility level of the organization (public, limited, private)
	Visibility VisibilityString `json:"visibility"`
	// Whether repository administrators can change team access
	RepoAdminChangeTeamAccess bool `json:"repo_admin_change_team_access"`
	// ID of the parent organization when this organization is a subgroup (0 for top-level organizations)
	ParentOrgID int64 `json:"parent_org_id,omitempty"`
	// Logical slash-delimited hierarchy path (e.g. "acme/platform/payments")
	FullPath string `json:"full_path,omitempty"`
	// username of the organization
	// deprecated
	UserName string `json:"username"`
}

// OrganizationPermissions list different users permissions on an organization
type OrganizationPermissions struct {
	// Whether the user is an owner of the organization
	IsOwner bool `json:"is_owner"`
	// Whether the user is an admin of the organization
	IsAdmin bool `json:"is_admin"`
	// Whether the user can write to the organization
	CanWrite bool `json:"can_write"`
	// Whether the user can read the organization
	CanRead bool `json:"can_read"`
	// Whether the user can create repositories in the organization
	CanCreateRepository bool `json:"can_create_repository"`
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	// username of the organization
	// required: true
	UserName string `json:"username" binding:"Required;Username;MaxSize(40)"`
	// The full display name of the organization
	FullName string `json:"full_name" binding:"MaxSize(100)"`
	// The email address of the organization
	Email string `json:"email" binding:"MaxSize(255)"`
	// The description of the organization
	Description string `json:"description" binding:"MaxSize(255)"`
	// The website URL of the organization
	Website string `json:"website" binding:"ValidUrl;MaxSize(255)"`
	// The location of the organization
	Location string `json:"location" binding:"MaxSize(50)"`
	// possible values are `public` (default), `limited` or `private`
	Visibility VisibilityString `json:"visibility" binding:"In(,public,limited,private)"`
	// Whether repository administrators can change team access
	RepoAdminChangeTeamAccess bool `json:"repo_admin_change_team_access"`
	// Optional parent organization username to create this organization as a child subgroup
	ParentOrg string `json:"parent_org,omitempty" binding:"MaxSize(40)"`
}

// CreateSubgroupOption options for creating a subgroup under an existing organization
type CreateSubgroupOption struct {
	// username of the subgroup organization
	// required: true
	UserName string `json:"username" binding:"Required;Username;MaxSize(40)"`
	// The full display name of the subgroup
	FullName string `json:"full_name" binding:"MaxSize(100)"`
	// The email address of the subgroup
	Email string `json:"email" binding:"MaxSize(255)"`
	// The description of the subgroup
	Description string `json:"description" binding:"MaxSize(255)"`
	// The website URL of the subgroup
	Website string `json:"website" binding:"ValidUrl;MaxSize(255)"`
	// The location of the subgroup
	Location string `json:"location" binding:"MaxSize(50)"`
	// possible values are `public`, `limited` or `private` (defaults to parent organization visibility)
	Visibility VisibilityString `json:"visibility" binding:"In(,public,limited,private)"`
	// Whether repository administrators can change team access
	RepoAdminChangeTeamAccess bool `json:"repo_admin_change_team_access"`
}

// MoveSubgroupOption options for moving or detaching an organization subgroup
type MoveSubgroupOption struct {
	// Username of the new parent organization; empty string detaches the subgroup to a top-level organization
	ParentOrg string `json:"parent_org" binding:"MaxSize(40)"`
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	// The full display name of the organization
	FullName *string `json:"full_name" binding:"MaxSize(100)"`
	// The email address of the organization; use empty string to clear
	Email *string `json:"email" binding:"MaxSize(255)"`
	// The description of the organization
	Description *string `json:"description" binding:"MaxSize(255)"`
	// The website URL of the organization
	Website *string `json:"website" binding:"ValidUrl;MaxSize(255)"`
	// The location of the organization
	Location *string `json:"location" binding:"MaxSize(50)"`
	// possible values are `public`, `limited` or `private`
	Visibility *VisibilityString `json:"visibility" binding:"In(,public,limited,private)"`
	// Whether repository administrators can change team access
	RepoAdminChangeTeamAccess *bool `json:"repo_admin_change_team_access"`
}

// RenameOrgOption options when renaming an organization
type RenameOrgOption struct {
	// New username for this org. This name cannot be in use yet by any other user.
	//
	// required: true
	// unique: true
	NewName string `json:"new_name" binding:"Required"`
}
