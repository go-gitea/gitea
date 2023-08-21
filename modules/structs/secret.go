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

// CreateSecretOption options when creating secret
// swagger:model
type CreateSecretOption struct {
	// Name of the secret to create
	//
	// required: true
	// unique: true
	Name string `json:"name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	// Data of the secret to create
	Data string `json:"data" binding:"Required"`
}
