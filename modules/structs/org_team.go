// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Team represents a team in an organization
type Team struct {
	ID                      int64         `json:"id"`
	Name                    string        `json:"name"`
	Description             string        `json:"description"`
	Organization            *Organization `json:"organization"`
	IncludesAllRepositories bool          `json:"includes_all_repositories"`
	// enum: none,read,write,admin,owner
	Permission string `json:"permission"`
	// example: ["repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.pulls","repo.releases","repo.ext_wiki"]
	Units            []string `json:"units"`
	CanCreateOrgRepo bool     `json:"can_create_org_repo"`
}

// CreateTeamOption options for creating a team
type CreateTeamOption struct {
	// required: true
	Name                    string `json:"name" binding:"Required;AlphaDashDot;MaxSize(30)"`
	Description             string `json:"description" binding:"MaxSize(255)"`
	IncludesAllRepositories bool   `json:"includes_all_repositories"`
	// enum: read,write,admin
	Permission string `json:"permission"`
	// example: ["repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.pulls","repo.releases","repo.ext_wiki"]
	Units            []string `json:"units"`
	CanCreateOrgRepo bool     `json:"can_create_org_repo"`
}

// EditTeamOption options for editing a team
type EditTeamOption struct {
	// required: true
	Name                    string `json:"name" binding:"Required;AlphaDashDot;MaxSize(30)"`
	Description             string `json:"description" binding:"MaxSize(255)"`
	IncludesAllRepositories bool   `json:"includes_all_repositories"`
	// enum: read,write,admin
	Permission string `json:"permission"`
	// example: ["repo.code","repo.issues","repo.ext_issues","repo.wiki","repo.pulls","repo.releases","repo.ext_wiki"]
	Units            []string `json:"units"`
	CanCreateOrgRepo bool     `json:"can_create_org_repo"`
}
