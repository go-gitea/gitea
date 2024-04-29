// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Compare represents a comparison between two commits.
type Compare struct {
	TotalCommits int       `json:"total_commits"` // Total number of commits in the comparison.
	Commits      []*Commit `json:"commits"`       // List of commits in the comparison.
}
