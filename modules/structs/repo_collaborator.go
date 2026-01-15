// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	// enum: read,write,admin
	// Permission level to grant the collaborator
	Permission *string `json:"permission"`
}

// RepoCollaboratorPermission to get repository permission for a collaborator
type RepoCollaboratorPermission struct {
	// Permission level of the collaborator
	Permission string `json:"permission"`
	// RoleName is the name of the permission role
	RoleName string `json:"role_name"`
	// User information of the collaborator
	User *User `json:"user"`
}
