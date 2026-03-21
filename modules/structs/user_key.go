// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// PublicKey publickey is a user key to push code to repository
type PublicKey struct {
	// ID is the unique identifier for the public key
	ID int64 `json:"id"`
	// Key contains the actual SSH public key content
	Key string `json:"key"`
	// URL is the API URL for this key
	URL string `json:"url,omitempty"`
	// Title is the human-readable name for the key
	Title string `json:"title,omitempty"`
	// Fingerprint is the key's fingerprint
	Fingerprint string `json:"fingerprint,omitempty"`
	// swagger:strfmt date-time
	// Created is the time when the key was added
	Created time.Time `json:"created_at"`
	// Updated is the time when the key was last used
	Updated time.Time `json:"last_used_at"`
	// Owner is the user who owns this key
	Owner *User `json:"user,omitempty"`
	// ReadOnly indicates if the key has read-only access
	ReadOnly bool `json:"read_only,omitempty"`
	// KeyType indicates the type of the SSH key
	KeyType string `json:"key_type,omitempty"`
}
