// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs // import "code.gitea.io/gitea/modules/structs"

// ExploreCodeSearchItem A single search match
// swagger:model
type ExploreCodeSearchItem struct {
	RepoName   string `json:"repoName"`
	FilePath   string `json:"path"`
	LineNumber int    `json:"lineNumber"`
	LineText   string `json:"lineText"`
}

// ExploreCodeResult all returned search results
// swagger:model
type ExploreCodeResult struct {
	Total   int                     `json:"total"`
	Results []ExploreCodeSearchItem `json:"results"`
}
