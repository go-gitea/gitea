// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// DeployKey a deploy key
type DeployKey struct {
	// ID is the unique identifier for the deploy key
	ID int64 `json:"id"`
	// KeyID is the associated public key ID
	KeyID int64 `json:"key_id"`
	// Key contains the actual SSH key content
	Key string `json:"key"`
	// URL is the API URL for this deploy key
	URL string `json:"url"`
	// Title is the human-readable name for the key
	Title string `json:"title"`
	// Fingerprint is the key's fingerprint
	Fingerprint string `json:"fingerprint"`
	// swagger:strfmt date-time
	// Created is the time when the deploy key was added
	Created time.Time `json:"created_at"`
	// ReadOnly indicates if the key has read-only access
	ReadOnly bool `json:"read_only"`
	// Repository is the repository this deploy key belongs to
	Repository *Repository `json:"repository,omitempty"`
}

// CreateKeyOption options when creating a key
type CreateKeyOption struct {
	// Title of the key to add
	//
	// required: true
	// unique: true
	Title string `json:"title" binding:"Required"`
	// An armored SSH key to add
	//
	// required: true
	// unique: true
	Key string `json:"key" binding:"Required"`
	// Describe if the key has only read access or read/write
	//
	// required: false
	ReadOnly bool `json:"read_only"`
}
