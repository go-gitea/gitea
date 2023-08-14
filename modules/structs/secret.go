// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// User represents a secret
// swagger:model
type Secret struct {
	// the secret's name
	Name string `json:"name"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
}
