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
	// glob patterns (comma-separated) that restrict which branches can access this environment's secrets and variables; empty means unrestricted
	ProtectedBranches string `json:"protected_branches"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateEnvironmentOption options for creating a new environment
// swagger:model
type CreateEnvironmentOption struct {
	// required: true
	Name string `json:"name" binding:"Required;MaxSize(255)"`
	// glob patterns (comma-separated) restricting which branches can access environment secrets and variables
	ProtectedBranches string `json:"protected_branches"`
}

// UpdateEnvironmentOption options for updating an environment
// swagger:model
type UpdateEnvironmentOption struct {
	// required: false
	Name string `json:"name"`
	// glob patterns (comma-separated) restricting which branches can access environment secrets and variables
	ProtectedBranches string `json:"protected_branches"`
}
