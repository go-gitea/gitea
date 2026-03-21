// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Team represents a team in an organization
type Team struct {
	// The unique identifier of the team
	ID int64 `json:"id"`
	// The name of the team
	Name string `json:"name"`
	// The description of the team
	Description string `json:"description"`
	// The organization that the team belongs to
	Organization *Organization `json:"organization"`
	// Whether the team has access to all repositories in the organization
	IncludesAllRepositories bool `json:"includes_all_repositories"`
	// enum: none,read,write,admin,owner
	Permission string `json:"permission"`
	// example: ["repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.pulls","repo.releases","repo.projects","repo.ext_wiki"]
	// Deprecated: This variable should be replaced by UnitsMap and will be dropped in later versions.
	Units []string `json:"units"`
	// example: {"repo.code":"read","repo.issues":"write","repo.ext_issues":"none","repo.wiki":"admin","repo.pulls":"owner","repo.releases":"none","repo.projects":"none","repo.ext_wiki":"none"}
	UnitsMap map[string]string `json:"units_map"`
	// Whether the team can create repositories in the organization
	CanCreateOrgRepo bool `json:"can_create_org_repo"`
}

// CreateTeamOption options for creating a team
type CreateTeamOption struct {
	// required: true
	Name string `json:"name" binding:"Required;AlphaDashDot;MaxSize(255)"`
	// The description of the team
	Description string `json:"description" binding:"MaxSize(255)"`
	// Whether the team has access to all repositories in the organization
	IncludesAllRepositories bool `json:"includes_all_repositories"`
	// enum: read,write,admin
	Permission string `json:"permission"`
	// example: ["repo.actions","repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.ext_wiki","repo.pulls","repo.releases","repo.projects","repo.ext_wiki"]
	// Deprecated: This variable should be replaced by UnitsMap and will be dropped in later versions.
	Units []string `json:"units"`
	// example: {"repo.actions","repo.packages","repo.code":"read","repo.issues":"write","repo.ext_issues":"none","repo.wiki":"admin","repo.pulls":"owner","repo.releases":"none","repo.projects":"none","repo.ext_wiki":"none"}
	UnitsMap map[string]string `json:"units_map"`
	// Whether the team can create repositories in the organization
	CanCreateOrgRepo bool `json:"can_create_org_repo"`
}

// EditTeamOption options for editing a team
type EditTeamOption struct {
	// required: true
	Name string `json:"name" binding:"AlphaDashDot;MaxSize(255)"`
	// The description of the team
	Description *string `json:"description" binding:"MaxSize(255)"`
	// Whether the team has access to all repositories in the organization
	IncludesAllRepositories *bool `json:"includes_all_repositories"`
	// enum: read,write,admin
	Permission string `json:"permission"`
	// example: ["repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.pulls","repo.releases","repo.projects","repo.ext_wiki"]
	// Deprecated: This variable should be replaced by UnitsMap and will be dropped in later versions.
	Units []string `json:"units"`
	// example: {"repo.code":"read","repo.issues":"write","repo.ext_issues":"none","repo.wiki":"admin","repo.pulls":"owner","repo.releases":"none","repo.projects":"none","repo.ext_wiki":"none"}
	UnitsMap map[string]string `json:"units_map"`
	// Whether the team can create repositories in the organization
	CanCreateOrgRepo *bool `json:"can_create_org_repo"`
}
