// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Environment represents a deployment environment
// swagger:model
type Environment struct {
	// ID of the environment
	ID int64 `json:"id"`
	// Name of the environment
	Name string `json:"name"`
	// Description of the environment  
	Description string `json:"description,omitempty"`
	// External URL associated with the environment
	ExternalURL string `json:"external_url,omitempty"`
	// Protection rules as JSON string
	ProtectionRules string `json:"protection_rules,omitempty"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	// User who created the environment
	CreatedBy *User `json:"created_by,omitempty"`
}

// CreateEnvironmentOption options for creating an environment
// swagger:model
type CreateEnvironmentOption struct {
	// required: true
	// Name of the environment
	Name string `json:"name" binding:"Required"`
	// Description of the environment
	Description string `json:"description,omitempty"`
	// External URL associated with the environment
	ExternalURL string `json:"external_url,omitempty"`
	// Protection rules as JSON string
	ProtectionRules string `json:"protection_rules,omitempty"`
}

// UpdateEnvironmentOption options for updating an environment
// swagger:model
type UpdateEnvironmentOption struct {
	// Description of the environment
	Description *string `json:"description,omitempty"`
	// External URL associated with the environment
	ExternalURL *string `json:"external_url,omitempty"`
	// Protection rules as JSON string
	ProtectionRules *string `json:"protection_rules,omitempty"`
}

// EnvironmentListResponse returns environments list
// swagger:model
type EnvironmentListResponse struct {
	// List of environments
	Environments []*Environment `json:"environments"`
	// Total number of environments
	TotalCount int64 `json:"total_count"`
}