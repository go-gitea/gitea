// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitHook represents a Git repository hook
type GitHook struct {
	// Name is the name of the Git hook
	Name string `json:"name"`
	// IsActive indicates if the hook is active
	IsActive bool `json:"is_active"`
	// Content contains the script content of the hook
	Content string `json:"content,omitempty"`
}

// GitHookList represents a list of Git hooks
type GitHookList []*GitHook

// EditGitHookOption options when modifying one Git hook
type EditGitHookOption struct {
	// Content is the new script content for the hook
	Content string `json:"content"`
}
