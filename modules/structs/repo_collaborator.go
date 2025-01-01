// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	// enum: read,write,admin
	Permission *string `json:"permission"`
}

// RepoCollaboratorPermission to get repository permission for a collaborator
type RepoCollaboratorPermission struct {
	Permission string `json:"permission"`
	RoleName   string `json:"role_name"`
	User       *User  `json:"user"`
}
