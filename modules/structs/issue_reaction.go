// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// EditReactionOption contain the reaction type
type EditReactionOption struct {
	// The reaction content (e.g., emoji or reaction type)
	Reaction string `json:"content"`
}

// Reaction contain one reaction
type Reaction struct {
	// The user who created the reaction
	User *User `json:"user"`
	// The reaction content (e.g., emoji or reaction type)
	Reaction string `json:"content"`
	// swagger:strfmt date-time
	// The date and time when the reaction was created
	Created time.Time `json:"created_at"`
}
