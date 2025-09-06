// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitEntry represents a git tree
type GitEntry struct {
	// Path is the file or directory path
	Path string `json:"path"`
	// Mode is the file mode (permissions)
	Mode string `json:"mode"`
	// Type indicates if this is a file, directory, or symlink
	Type string `json:"type"`
	// Size is the file size in bytes
	Size int64 `json:"size"`
	// SHA is the Git object SHA
	SHA string `json:"sha"`
	// URL is the API URL for this tree entry
	URL string `json:"url"`
}

// GitTreeResponse returns a git tree
type GitTreeResponse struct {
	// SHA is the tree object SHA
	SHA string `json:"sha"`
	// URL is the API URL for this tree
	URL string `json:"url"`
	// Entries contains the tree entries (files and directories)
	Entries []GitEntry `json:"tree"`
	// Truncated indicates if the response was truncated due to size
	Truncated bool `json:"truncated"`
	// Page is the current page number for pagination
	Page int `json:"page"`
	// TotalCount is the total number of entries in the tree
	TotalCount int `json:"total_count"`
}
