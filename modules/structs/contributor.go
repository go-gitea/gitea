// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Contributor represents a repository contributor.
// swagger:model
type Contributor struct {
	// user login name, used by non-git only user
	Login string `json:"login"`
	// user id, used by non-git only user
	ID int64 `json:"id"`
	// URL to the user's avatar, used by non-git only user
	AvatarURL string `json:"avatar_url"`
	// URL to the user's gitea page, used by non-git only user
	HTMLURL string `json:"html_url"`
	// Name of the contributor
	Name string `json:"name,omitempty"`
	// Email of the contributor
	Email string `json:"email,omitempty"`
	// Contributions is the number of commits made by the contributor for Github API compatibility
	Contributions int64 `json:"contributions"`
	// Additions is the number of lines added by the contributor
	Additions int64 `json:"additions"`
	// Deletions is the number of lines deleted by the contributor
	Deletions int64 `json:"deletions"`
	// Commits is the number of commits made by the contributor
	Commits int64 `json:"commits"`
	// FilesChanged is the number of files changed by the contributor
	FilesChanged int64 `json:"fileschanged"`
}
