// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CreateVariableOption the option when creating variable
// swagger:model
type CreateVariableOption struct {
	// Value of the variable to create
	//
	// required: true
	Value string `json:"value" binding:"Required"`
}

// UpdateVariableOption the option when updating variable
// swagger:model
type UpdateVariableOption struct {
	// New name for the variable. If the field is empty, the variable name won't be updated.
	Name string `json:"name"`
	// Value of the variable to update
	//
	// required: true
	Value string `json:"value" binding:"Required"`
}

// ActionVariable return value of the query API
// swagger:model
type ActionVariable struct {
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
	Name    string `json:"name"`
	Data    string `json:"data"`
}
