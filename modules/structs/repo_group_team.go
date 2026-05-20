// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CreateOrUpdateRepoGroupTeamOption options for adding a team to a repo group
type CreateOrUpdateRepoGroupTeamOption struct {
	// Whether the team can create repositories and subgroups in the group
	CanCreateIn *bool `json:"can_create_in"`
	// example: {"repo.code":"read","repo.issues":"write","repo.ext_issues":"none","repo.wiki":"admin","repo.pulls":"owner","repo.releases":"none","repo.projects":"none","repo.ext_wiki":"none"}
	UnitsMap   map[string]string    `json:"units_map"`
	Permission *RepoWritePermission `json:"permission"`
}
