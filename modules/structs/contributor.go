// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Contributor represents a repository contributor.
// swagger:model
type Contributor struct {
	*User `json:",inline"`
	// Name of the contributor, used for anonymous contributors
	Name string `json:"name,omitempty"`
	// Email of the contributor, used for anonymous contributors
	Email string `json:"email,omitempty"`
	// Contributions is the number of commits made by the contributor
	Contributions int64 `json:"contributions"`
}
