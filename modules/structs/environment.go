// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// ActionEnvironment represents a deployment environment
// swagger:model
type ActionEnvironment struct {
	// the environment's id
	ID int64 `json:"id"`
	// the environment's name
	Name string `json:"name"`
	// branch globs, or "refs/tags/<glob>" for tags, allowed to deploy; empty allows every ref
	AllowedBranchPatterns []string `json:"allowed_branch_patterns"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateOrUpdateEnvironmentOption options for creating or updating a deployment environment
// swagger:model
type CreateOrUpdateEnvironmentOption struct {
	// branch globs, or "refs/tags/<glob>" for tags, allowed to deploy; omitted or empty replaces the policy with allowing every ref
	AllowedBranchPatterns []string `json:"allowed_branch_patterns"`
}
