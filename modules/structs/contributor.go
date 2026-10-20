// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Contributor represents a commit author of the default branch of a repository
// swagger:model
type Contributor struct {
	// "User" or "Bot" for authors matched to a user, otherwise "Anonymous"
	Type string `json:"type"`
	// not set for type "Anonymous"
	ID int64 `json:"id,omitzero"`
	// not set for type "Anonymous"
	Login string `json:"login,omitzero"`
	// not set for type "Anonymous"
	AvatarURL string `json:"avatar_url,omitzero"`
	// not set for type "Anonymous"
	HTMLURL string `json:"html_url,omitzero"`
	// the git author name, only set for type "Anonymous"
	Name string `json:"name,omitzero"`
	// the git author email, only set for type "Anonymous"
	Email string `json:"email,omitzero"`
	// number of commits, merges included
	Contributions int64 `json:"contributions"`
	Additions     int64 `json:"additions"`
	Deletions     int64 `json:"deletions"`
}
