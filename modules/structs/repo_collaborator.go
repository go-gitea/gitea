// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// RepoWritePermission is a permission level callers may grant to a team or
// collaborator on input. Output fields use AccessLevelName instead.
// swagger:enum RepoWritePermission
type RepoWritePermission string

const (
	RepoWritePermissionRead  RepoWritePermission = "read"
	RepoWritePermissionWrite RepoWritePermission = "write"
	RepoWritePermissionAdmin RepoWritePermission = "admin"
)

// AccessLevelName is the string rendering of a perm.AccessMode produced on
// API responses. Callers must not send these values; use RepoWritePermission
// on input.
// swagger:enum AccessLevelName
type AccessLevelName string

const (
	AccessLevelNameNone  AccessLevelName = "none"
	AccessLevelNameRead  AccessLevelName = "read"
	AccessLevelNameWrite AccessLevelName = "write"
	AccessLevelNameAdmin AccessLevelName = "admin"
	AccessLevelNameOwner AccessLevelName = "owner"
)

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	// Permission level to grant the collaborator
	Permission *RepoWritePermission `json:"permission"`
}

// RepoCollaboratorPermission to get repository permission for a collaborator
type RepoCollaboratorPermission struct {
	// Permission level of the collaborator
	Permission AccessLevelName `json:"permission"`
	// RoleName is the name of the permission role
	RoleName string `json:"role_name"`
	// User information of the collaborator
	User *User `json:"user"`
}
