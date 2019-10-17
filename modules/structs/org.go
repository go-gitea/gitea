// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Organization represents the brief API response fields for an Organization
// swagger:model
type Organization struct {
	// the org's id
	ID int64 `json:"id"`
	// the org's login
	Login string `json:"login"`
	// the org's full name
	FullName string `json:"full_name"`
	// URL to the org's avatar
	AvatarURL string `json:"avatar_url"`
	// URL to the org's API endpoint
	URL string `json:"url"`
	// URL to the org's Gitea HTML page
	HTMLURL string `json:"html_url"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated,omitempty"`
}

// OrganizationDetails represents all the API response fields for an Organization
// swagger:model
type OrganizationDetails struct {
	// the org's id
	ID int64 `json:"id"`
	// the org's username
	UserName string `json:"username"`
	// the org's full name
	FullName string `json:"full_name"`
	// URL to the org's avatar
	AvatarURL string `json:"avatar_url"`
	// Biography about the user
	Description string `json:"bio"`
	// Website of the user
	Website string `json:"website"`
	// Location of the user
	Location string `json:"location"`
	// URL to the org's API endpoint
	URL string `json:"url"`
	// URL to the org's Gitea HTML page
	HTMLURL string `json:"html_url"`
	// URL to the org's repos API endpoint
	ReposURL string `json:"repos_url"`
	// URL to org's members API endpoint
	MembersURL string `json:"members_url"`
	// URL to org's public members API endpoint
	PublicMembersURL string `json:"public_members_url"`
	// URL to org's teams API endpoint
	TeamsURL string `json:"teams_url"`
	// URL to org's hooks API endpoint
	HooksURL string `json:"hooks_url"`
	// The org's followers count
	Followers int `json:"followers"`
	// The org's following count
	Following int `json:"following"`
	// The org's public repo count
	PublicRepos int64 `json:"public_repos"`
	// Type
	Type string `json:"type"`
	// Visibility of the organization
	Visibility string `json:"visibility"`
	// Repo admin can change team access
	RepoAdminChangeTeamAccess bool `json:"repo_admin_change_team_access"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated,omitempty"`
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	// required: true
	UserName    string `json:"username" binding:"Required"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	// possible values are `public` (default), `limited` or `private`
	// enum: public,limited,private
	Visibility                string `json:"visibility" binding:"In(,public,limited,private)"`
	RepoAdminChangeTeamAccess bool   `json:"repo_admin_change_team_access"`
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	// possible values are `public`, `limited` or `private`
	// enum: public,limited,private
	Visibility                string `json:"visibility" binding:"In(,public,limited,private)"`
	RepoAdminChangeTeamAccess bool   `json:"repo_admin_change_team_access"`
}
