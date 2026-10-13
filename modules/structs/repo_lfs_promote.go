// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// PromoteToLFSOptions names the files to move into Git LFS.
type PromoteToLFSOptions struct {
	// Repository paths, on the branch below. A path already stored as a
	// pointer is skipped rather than refused.
	// required: true
	Files []string `json:"files" binding:"Required"`
	// The branch to read the blobs from and commit the pointers to. Defaults
	// to the repository's default branch. One branch, because promoting is a
	// commit and a commit is on a branch.
	Branch  string `json:"branch"`
	Message string `json:"message"`
}

// PromoteToLFSResponse says which paths were moved.
type PromoteToLFSResponse struct {
	// Empty when every path named was already stored as a pointer, which is a
	// success with nothing to do rather than a failure.
	Moved []string `json:"moved"`
}
