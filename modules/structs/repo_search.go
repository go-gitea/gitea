// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CodeSearchResultLanguage result of top languages count in search results
type CodeSearchResultLanguage struct {
	Language string
	Color    string
	Count    int
}

type CodeSearchResultLine struct {
	LineNumber int    `json:"line_number"`
	RawContent string `json:"raw_content"`
}

type CodeSearchResult struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Language   string `json:"language"`
	Color      string
	Lines      []CodeSearchResultLine
	Sha        string      `json:"sha"`
	URL        string      `json:"url"`
	HTMLURL    string      `json:"html_url"`
	Repository *Repository `json:"repository"`
}

type CodeSearchResults struct {
	TotalCount int64                      `json:"total_count"`
	Items      []CodeSearchResult         `json:"items"`
	Languages  []CodeSearchResultLanguage `json:"languages,omitempty"`
}
