// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitBlobResponse represents a git blob
type GitBlobResponse struct {
	// The content of the git blob (may be base64 encoded)
	Content *string `json:"content"`
	// The encoding used for the content (e.g., "base64")
	Encoding *string `json:"encoding"`
	// The URL to access this git blob
	URL string `json:"url"`
	// The SHA hash of the git blob
	SHA string `json:"sha"`
	// The size of the git blob in bytes
	Size int64 `json:"size"`

	// The LFS object ID if this blob is stored in LFS
	LfsOid *string `json:"lfs_oid,omitempty"`
	// The size of the LFS object if this blob is stored in LFS
	LfsSize *int64 `json:"lfs_size,omitempty"`
}
