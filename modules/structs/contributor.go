// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Contributor represents a commit author of the default branch of a repository
// swagger:model
type Contributor struct {
	// "User" for authors matched to a user, otherwise "Anonymous"
	Type string `json:"type"`
	// only set for type "User"
	ID int64 `json:"id,omitzero"`
	// only set for type "User"
	Login string `json:"login,omitzero"`
	// only set for type "User"
	AvatarURL string `json:"avatar_url,omitzero"`
	// only set for type "User"
	HTMLURL string `json:"html_url,omitzero"`
	// the git author name, only set for type "Anonymous"
	Name string `json:"name,omitzero"`
	// the git author email, only set for type "Anonymous"
	Email string `json:"email,omitzero"`
	// number of non-merge commits
	Contributions int64 `json:"contributions"`
	// number of lines added by these commits
	Additions int64 `json:"additions"`
	// number of lines deleted by these commits
	Deletions int64 `json:"deletions"`
}
