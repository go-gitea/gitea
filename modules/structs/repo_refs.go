// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Reference represents a Git reference.
type Reference struct {
	// The name of the Git reference (e.g., refs/heads/main)
	Ref string `json:"ref"`
	// The URL to access this Git reference
	URL string `json:"url"`
	// The Git object that this reference points to
	Object *GitObject `json:"object"`
}

// GitObject represents a Git object.
type GitObject struct {
	// The type of the Git object (e.g., commit, tag, tree, blob)
	Type string `json:"type"`
	// The SHA hash of the Git object
	SHA string `json:"sha"`
	// The URL to access this Git object
	URL string `json:"url"`
}
