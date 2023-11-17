// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// Secret represents a secret
// swagger:model
type Secret struct {
	// the secret's name
	Name string `json:"name"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
}

// CreateOrUpdateSecretOption options when creating or updating secret
// swagger:model
type CreateOrUpdateSecretOption struct {
	// Data of the secret to update
	//
	// required: true
	Data string `json:"data" binding:"Required"`
}
