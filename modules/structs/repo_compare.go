// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitCompareResponse returns a git compare result
type GitCompareResponse struct {
	URL             string            `json:"url"`
	HTMLURL         string            `json:"html_url"`
	PermalinkURL    string            `json:"permalink_url"`
	BaseCommit      *Commit           `json:"base_commit,omitempty"`
	MergeBaseCommit *Commit           `json:"merge_base_commit,omitempty"`
	AheadBy         int64             `json:"ahead_by"`
	BehindBy        int64             `json:"behind_by"`
	TotalCommits    int64             `json:"total_commits"`
	Commits         []*Commit         `json:"commits"`
	Files           []*GitCompareFile `json:"files"`
}

// GitCompareFile response one changed file
type GitCompareFile struct {
	SHA         string `json:"sha"`
	FileName    string `json:"filename"`
	OldFileName string `json:"old_filename"`
	Status      string `json:"status"`
	Additions   int64  `json:"additions"`
	Deletions   int64  `json:"deletions"`
	Changes     int64  `json:"changes"`
	Patch       string `json:"patch,omitempty"`
}
