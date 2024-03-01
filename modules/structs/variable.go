// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CreateVariableOption the option when creating variable
// swagger:model
type CreateVariableOption struct {
	// Value of the variable to create
	// required: true
	Value string `json:"value" binding:"Required"`
}

// UpdateVariableOption the option when updating variable
type UpdateVariableOption struct {
	// New name for the variable. If the field is empty, the variable name won't be updated.
	Name string `json:"name"`
	// Value of the variable to update
	// required: true
	Value string `json:"value" binding:"Required"`
}
