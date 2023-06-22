// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	Permission *string `json:"permission"`
}

// RepoCollaboratorPermission to get repository permission for a collaborator
type RepoCollaboratorPermission struct {
	Permission string `json:"permission"`
	RoleName   string `json:"role_name"`
	User       *User  `json:"user"`
}

type WeekData struct {
	Week      int64 `json:"week"`
	Additions int   `json:"additions"`
	Deletions int   `json:"deletions"`
	Commits   int   `json:"commits"`
}

// ContributorData represents statistical git commit count data
type ContributorData struct {
	Name         string      `json:"name"`
	Login        string      `json:"login"`
	AvatarLink   string      `json:"avatar_link"`
	HomeLink     string      `json:"home_link"`
	TotalCommits int64       `json:"total_commits"`
	Weeks        []*WeekData `json:"weeks"`
}
