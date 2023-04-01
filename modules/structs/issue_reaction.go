// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// EditReactionOption contain the reaction type
type EditReactionOption struct {
	Reaction string `json:"content"`
}

// Reaction contain one reaction
type Reaction struct {
	User     *User  `json:"user"`
	Reaction string `json:"content"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
}
