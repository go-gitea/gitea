// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitHook represents a Git repository hook
type GitHook struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Content  string `json:"content,omitempty"`
}

// GitHookList represents a list of Git hooks
type GitHookList []*GitHook

// EditGitHookOption options when modifying one Git hook
type EditGitHookOption struct {
	Content string `json:"content"`
}
