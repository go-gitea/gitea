// Copyright 2024 The Gitea Authors. All rights reserved.
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
	// glob patterns (comma-separated) that restrict which branches can deploy here; empty means unrestricted
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
	// glob patterns (comma-separated) restricting deployable branches
	ProtectedBranches string `json:"protected_branches"`
}

// UpdateEnvironmentOption options for updating an environment
// swagger:model
type UpdateEnvironmentOption struct {
	// required: false
	Name string `json:"name"`
	// glob patterns (comma-separated) restricting deployable branches
	ProtectedBranches string `json:"protected_branches"`
}

// EnvironmentSecret represents a secret scoped to an environment (value is never returned)
// swagger:model
type EnvironmentSecret struct {
	// the secret's name
	Name string `json:"name"`
	// the secret's description
	Description string `json:"description"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
}

// CreateOrUpdateEnvironmentSecretOption options for creating or updating an environment secret
// swagger:model
type CreateOrUpdateEnvironmentSecretOption struct {
	// required: true
	Data string `json:"data" binding:"Required"`
	// required: false
	Description string `json:"description"`
}

// EnvironmentVariable represents a variable scoped to an environment
// swagger:model
type EnvironmentVariable struct {
	// the variable's name
	Name string `json:"name"`
	// the variable's value
	Value string `json:"value"`
	// the variable's description
	Description string `json:"description"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
}

// CreateEnvironmentVariableOption options for creating an environment variable
// swagger:model
type CreateEnvironmentVariableOption struct {
	// required: true
	Value string `json:"value" binding:"Required"`
	// required: false
	Description string `json:"description"`
}

// UpdateEnvironmentVariableOption options for updating an environment variable
// swagger:model
type UpdateEnvironmentVariableOption struct {
	// required: false
	Name string `json:"name"`
	// required: false
	Value string `json:"value"`
	// required: false
	Description string `json:"description"`
}
